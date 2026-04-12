package browserfetch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
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
	if f == nil || f.browser == nil {
		return nil, fmt.Errorf("browserfetch: nil RodFetcher")
	}
	u, err := requireAbsoluteHTTPS(rawURL)
	if err != nil {
		return nil, err
	}
	page, err := f.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, fmt.Errorf("browserfetch: new page: %w", err)
	}
	defer func() { _ = page.Close() }()

	p := page.Context(ctx).Timeout(f.navTimeout)

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
		return nil, fmt.Errorf("browserfetch: navigate: %w", err)
	}
	wait()
	waitCalled = true

	if err := sleepOrCtxDone(p.GetContext(), postLoadSettle); err != nil {
		return nil, fmt.Errorf("browserfetch: settle after load: %w", err)
	}
	html, err := p.HTML()
	if err != nil {
		return nil, fmt.Errorf("browserfetch: read html: %w", err)
	}
	return []byte(html), nil
}

// Close shuts down the browser process.
func (f *RodFetcher) Close() error {
	if f == nil || f.browser == nil {
		return nil
	}
	return f.browser.Close()
}
