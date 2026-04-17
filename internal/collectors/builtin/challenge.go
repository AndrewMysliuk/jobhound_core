package builtin

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// challengeHTMLScanCap bounds how much of a response we scan for interstitial markers
// (real job pages are larger; challenge HTML exposes markers early).
const challengeHTMLScanCap = 96 << 10

// looksLikeCloudflareChallengeHTML reports HTML shaped like a Cloudflare browser check page.
// Markers are intentionally narrow — see specs/005-job-collectors/tasks.md § N.
func looksLikeCloudflareChallengeHTML(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	scan := b
	if len(scan) > challengeHTMLScanCap {
		scan = scan[:challengeHTMLScanCap]
	}
	low := strings.ToLower(string(scan))
	if strings.Contains(low, "cdn-cgi/challenge-platform") {
		return true
	}
	if strings.Contains(low, "cf-browser-verification") {
		return true
	}
	// Title-only interstitials still pull scripts from cdn-cgi; avoid lone "just a moment" matches.
	if strings.Contains(low, "just a moment") && strings.Contains(low, "cdn-cgi") {
		return true
	}
	return false
}

func sleepForCtx(ctx context.Context, d time.Duration) error {
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

func (c *BuiltIn) challengeBackoffSchedule() []time.Duration {
	if c != nil && len(c.challengeRetryDelays) > 0 {
		return c.challengeRetryDelays
	}
	return []time.Duration{5 * time.Second}
}

// fetchDocumentWithChallengeRetries loads the same URL on the same transport path (HTTP vs Rod).
// After an interstitial-shaped body, waits 5s by default (overridable for tests), refetches once,
// then returns the last body (at most two successful transport responses per URL).
func (c *BuiltIn) fetchDocumentWithChallengeRetries(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	body, err := c.fetchDocument(ctx, client, rawURL)
	if err != nil {
		return nil, err
	}
	if !looksLikeCloudflareChallengeHTML(body) {
		return body, nil
	}
	for _, wait := range c.challengeBackoffSchedule() {
		if err := sleepForCtx(ctx, wait); err != nil {
			return nil, err
		}
		body, err = c.fetchDocument(ctx, client, rawURL)
		if err != nil {
			return nil, err
		}
		if !looksLikeCloudflareChallengeHTML(body) {
			return body, nil
		}
	}
	return body, nil
}
