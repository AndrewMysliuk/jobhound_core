package browserfetch

import (
	"context"
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
	opts       RodOptions
	pageMu     sync.Mutex
}

// NewRodFetcher launches Chromium and connects. It fails fast if the browser cannot start.
func NewRodFetcher(opts RodOptions) (*RodFetcher, error) {
	nav := opts.NavTimeout
	if nav <= 0 {
		nav = 2 * time.Minute
	}
	browser, err := launchBrowser(opts)
	if err != nil {
		return nil, err
	}
	return &RodFetcher{browser: browser, navTimeout: nav, opts: opts}, nil
}

func launchBrowser(opts RodOptions) (*rod.Browser, error) {
	launchCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// enable-automation makes Cloudflare serve a block page with no job payload.
	l := launcher.New().Context(launchCtx).Headless(true).
		Delete("enable-automation").
		Set("disable-blink-features", "AutomationControlled").
		Set("user-agent", utils.BrowserUserAgent)
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
	return browser, nil
}

func (f *RodFetcher) relaunch() error {
	if f.browser != nil {
		_ = f.browser.Close()
		f.browser = nil
	}
	browser, err := launchBrowser(f.opts)
	if err != nil {
		return err
	}
	f.browser = browser
	return nil
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
	doc, err := f.navigateLocked(ctx, rawURL, reportURL)
	if err != nil || !cloudflareBlocked(doc.HTML) {
		return doc, err
	}
	if err := f.relaunch(); err != nil {
		return NavigatedDocument{}, err
	}
	return f.navigateLocked(ctx, rawURL, reportURL)
}

func (f *RodFetcher) navigateLocked(ctx context.Context, rawURL string, reportURL bool) (NavigatedDocument, error) {
	if f == nil || f.browser == nil {
		return NavigatedDocument{}, fmt.Errorf("browserfetch: nil RodFetcher")
	}
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
		if err := p.SetUserAgent(browserUserAgentOverride()); err != nil {
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
	html, err = waitPastChallenge(p, html)
	if err != nil {
		return NavigatedDocument{}, err
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

// Close shuts down the browser process.
func (f *RodFetcher) Close() error {
	if f == nil || f.browser == nil {
		return nil
	}
	return f.browser.Close()
}

func browserUserAgentOverride() *proto.NetworkSetUserAgentOverride {
	return &proto.NetworkSetUserAgentOverride{
		UserAgent: utils.BrowserUserAgent,
		Platform:  "Win32",
		UserAgentMetadata: &proto.EmulationUserAgentMetadata{
			Brands: []*proto.EmulationUserAgentBrandVersion{
				{Brand: "Chromium", Version: "131"},
				{Brand: "Google Chrome", Version: "131"},
				{Brand: "Not_A Brand", Version: "24"},
			},
			FullVersionList: []*proto.EmulationUserAgentBrandVersion{
				{Brand: "Chromium", Version: "131.0.0.0"},
				{Brand: "Google Chrome", Version: "131.0.0.0"},
				{Brand: "Not_A Brand", Version: "24.0.0.0"},
			},
			Platform:        "Windows",
			PlatformVersion: "15.0.0",
			Architecture:    "x86",
			Model:           "",
			Mobile:          false,
			Bitness:         "64",
		},
	}
}

func waitPastChallenge(p *rod.Page, html string) (string, error) {
	if !cloudflareChallenge(html) {
		return html, nil
	}
	for range 12 {
		if err := sleepOrCtxDone(p.GetContext(), time.Second); err != nil {
			return "", fmt.Errorf("browserfetch: cloudflare challenge: %w", err)
		}
		next, err := p.HTML()
		if err != nil {
			return "", fmt.Errorf("browserfetch: read html: %w", err)
		}
		html = next
		if !cloudflareChallenge(html) {
			return html, nil
		}
	}
	return html, nil
}

func cloudflareChallenge(html string) bool {
	return strings.Contains(html, "Just a moment") || strings.Contains(html, "cf-challenge")
}

func cloudflareBlocked(html []byte) bool {
	s := string(html)
	return strings.Contains(s, "cf-error-details") || strings.Contains(s, "Sorry, you have been blocked") || strings.Contains(s, "Just a moment")
}
