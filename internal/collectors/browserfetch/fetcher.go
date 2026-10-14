// Package browserfetch provides source-agnostic HTTPS document loads via a headless browser (go-rod).
// It is shared infrastructure, not a collectors.Collector — see specs/005-job-collectors/contracts/browser-fetch.md.
package browserfetch

import "context"

// HTMLDocumentFetcher loads the final HTML document for an absolute HTTPS URL after browser navigation.
type HTMLDocumentFetcher interface {
	FetchHTMLDocument(ctx context.Context, rawURL string) ([]byte, error)
}

// NavigatedDocument is the HTML after navigation and the URL the browser landed on.
type NavigatedDocument struct {
	FinalURL string
	HTML     []byte
}

// NavigatedFetcher loads one absolute HTTPS URL and reports the post-redirect URL.
type NavigatedFetcher interface {
	FetchNavigated(ctx context.Context, rawURL string) (NavigatedDocument, error)
}

// RawFetcher loads the original document response. The live DOM is not used.
type RawFetcher interface {
	FetchRaw(ctx context.Context, rawURL string) (NavigatedDocument, error)
}
