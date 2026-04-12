// Package browserfetch provides source-agnostic HTTPS document loads via a headless browser (go-rod).
// It is shared infrastructure, not a collectors.Collector — see specs/005-job-collectors/contracts/browser-fetch.md.
package browserfetch

import "context"

// HTMLDocumentFetcher loads the final HTML document for an absolute HTTPS URL after browser navigation.
type HTMLDocumentFetcher interface {
	FetchHTMLDocument(ctx context.Context, rawURL string) ([]byte, error)
}
