package browserfetch

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

// RodOptions configures headless Chromium launched for [RodFetcher].
type RodOptions struct {
	// Bin is the Chromium/Chrome executable path; empty uses launcher defaults (may auto-download).
	Bin string
	// UserDataDir is an optional persistent profile directory; empty uses launcher default temp dir.
	UserDataDir string
	// NavTimeout bounds each navigation + load + HTML extraction for one [RodFetcher.FetchHTMLDocument] call.
	NavTimeout time.Duration
	// NoSandbox passes --no-sandbox (required for Chromium as root in Docker). Do not use on untrusted multi-tenant hosts.
	NoSandbox bool
}

// RodFetcher implements [HTMLDocumentFetcher] with one long-lived Chromium process and a fresh tab per request.
// Ops: a single browser per process reduces startup cost versus launching Chromium on every URL; each call opens
// and closes one page. Call [RodFetcher.Close] on shutdown to release the browser (optional for short-lived CLIs).
type RodFetcher struct {
	browser    *rod.Browser
	navTimeout time.Duration
	pageMu     sync.Mutex
}

// NewRodFetcher launches Chromium and connects. It fails fast if the browser cannot start.
func NewRodFetcher(opts RodOptions) (*RodFetcher, error) {
	nav := opts.NavTimeout
	if nav <= 0 {
		nav = 2 * time.Minute
	}
	launchCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	l := launcher.New().Context(launchCtx).Headless(true)
	if b := strings.TrimSpace(opts.Bin); b != "" {
		l = l.Bin(b)
	}
	if d := strings.TrimSpace(opts.UserDataDir); d != "" {
		l = l.UserDataDir(d)
	}
	if opts.NoSandbox {
		l = l.NoSandbox(true).Set("disable-dev-shm-usage", "true")
	}
	wsURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("browserfetch: launch chromium: %w", err)
	}
	browser := rod.New().ControlURL(wsURL).Context(context.Background())
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("browserfetch: connect to chromium: %w", err)
	}
	return &RodFetcher{browser: browser, navTimeout: nav}, nil
}

// postLoadSettle is a fixed pause after the lifecycle "load" event. WaitRequestIdle+WaitDOMStable
// (Rod WaitStable without WaitLoad) can run until navTimeout on sites with endless XHR or DOM churn.
const postLoadSettle = 750 * time.Millisecond

func sleepOrCtxDone(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// FetchHTMLDocument navigates to rawURL, waits for load, then returns the document HTML.
func (f *RodFetcher) FetchHTMLDocument(ctx context.Context, rawURL string) ([]byte, error) {
	doc, err := f.navigate(ctx, rawURL, false)
	if err != nil {
		return nil, err
	}
	return doc.HTML, nil
}

// FetchNavigated navigates to rawURL and returns the HTML plus the URL after redirects.
// It sends the shared browser User-Agent. FetchHTMLDocument does not.
func (f *RodFetcher) FetchNavigated(ctx context.Context, rawURL string) (NavigatedDocument, error) {
	return f.navigate(ctx, rawURL, true)
}

func (f *RodFetcher) navigate(ctx context.Context, rawURL string, reportURL bool) (NavigatedDocument, error) {
	if f == nil || f.browser == nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: nil RodFetcher")
	}
	f.pageMu.Lock()
	defer f.pageMu.Unlock()
	if err := ctx.Err(); err != nil {
		return NavigatedDocument{}, err
	}
	u, err := requireAbsoluteHTTPS(rawURL)
	if err != nil {
		return NavigatedDocument{}, err
	}
	page, err := f.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: new page: %w", err)
	}
	defer func() { _ = page.Close() }()

	p := page.Context(ctx).Timeout(f.navTimeout)
	if reportURL {
		if err := p.SetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: utils.BrowserUserAgent}); err != nil {
			return NavigatedDocument{}, fmt.Errorf("browserfetch: user agent: %w", err)
		}
	}

	// Wait for window "load" via Page lifecycle events (CDP), not WaitLoad's JS eval path,
	// which can fail on some origins with {-32000 Object reference chain is too long}.
	wait := p.WaitNavigation(proto.PageLifecycleEventNameLoad)
	waitCalled := false
	defer func() {
		if !waitCalled {
			_ = proto.PageSetLifecycleEventsEnabled{Enabled: false}.Call(p)
		}
	}()

	if err := p.Navigate(u); err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: navigate: %w", err)
	}
	wait()
	waitCalled = true

	if err := sleepOrCtxDone(p.GetContext(), postLoadSettle); err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: settle after load: %w", err)
	}
	html, err := p.HTML()
	if err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: read html: %w", err)
	}
	doc := NavigatedDocument{HTML: []byte(html)}
	if !reportURL {
		return doc, nil
	}
	info, err := p.Info()
	if err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: page url: %w", err)
	}
	doc.FinalURL = info.URL
	return doc, nil
}

// FetchRaw returns the document response body and the URL after redirects.
// p.HTML is the DOM after scripts run, so a value removed at hydration is still in this body.
func (f *RodFetcher) FetchRaw(ctx context.Context, rawURL string) (NavigatedDocument, error) {
	if f == nil || f.browser == nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: nil RodFetcher")
	}
	f.pageMu.Lock()
	defer f.pageMu.Unlock()
	if err := ctx.Err(); err != nil {
		return NavigatedDocument{}, err
	}
	u, err := requireAbsoluteHTTPS(rawURL)
	if err != nil {
		return NavigatedDocument{}, err
	}
	page, err := f.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: new page: %w", err)
	}
	defer func() { _ = page.Close() }()

	p := page.Context(ctx).Timeout(f.navTimeout)
	if err := p.SetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: utils.BrowserUserAgent}); err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: user agent: %w", err)
	}
	if err := (proto.PageSetLifecycleEventsEnabled{Enabled: true}).Call(p); err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: lifecycle: %w", err)
	}

	var (
		last       proto.NetworkRequestID
		lastStatus int
		saw        bool
		body       []byte
		bodyErr    error
	)
	wait := p.EachEvent(
		func(e *proto.NetworkResponseReceived) {
			if e.Type != proto.NetworkResourceTypeDocument || e.Response == nil {
				return
			}
			if e.Response.Status >= 300 && e.Response.Status < 400 {
				return
			}
			last = e.RequestID
			lastStatus = e.Response.Status
			saw = true
		},
		func(e *proto.PageLifecycleEvent) bool {
			if e.Name != proto.PageLifecycleEventNameLoad {
				return false
			}
			if !saw {
				bodyErr = fmt.Errorf("browserfetch: document response missing")
				return true
			}
			if lastStatus < 200 || lastStatus >= 300 {
				bodyErr = fmt.Errorf("browserfetch: document status %d", lastStatus)
				return true
			}
			res, err := (proto.NetworkGetResponseBody{RequestID: last}).Call(p)
			if err != nil {
				bodyErr = fmt.Errorf("browserfetch: response body: %w", err)
				return true
			}
			decoded, err := decodeResponseBody(res)
			if err != nil {
				bodyErr = fmt.Errorf("browserfetch: response body: %w", err)
				return true
			}
			if len(decoded) == 0 {
				bodyErr = fmt.Errorf("browserfetch: empty document body")
				return true
			}
			body = decoded
			return true
		},
	)
	if err := p.Navigate(u); err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: navigate: %w", err)
	}
	wait()
	if bodyErr != nil {
		return NavigatedDocument{}, bodyErr
	}
	if err := p.GetContext().Err(); err != nil {
		return NavigatedDocument{}, err
	}
	if len(body) == 0 {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: empty document body")
	}
	info, err := p.Info()
	if err != nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: page url: %w", err)
	}
	if strings.TrimSpace(info.URL) == "" {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: empty final URL")
	}
	return NavigatedDocument{FinalURL: info.URL, HTML: body}, nil
}

func decodeResponseBody(res *proto.NetworkGetResponseBodyResult) ([]byte, error) {
	if res == nil {
		return nil, fmt.Errorf("nil result")
	}
	if !res.Base64Encoded {
		return []byte(res.Body), nil
	}
	return base64.StdEncoding.DecodeString(res.Body)
}

// Close shuts down the browser process.
func (f *RodFetcher) Close() error {
	if f == nil || f.browser == nil {
		return nil
	}
	return f.browser.Close()
}
