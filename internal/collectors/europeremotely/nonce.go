package europeremotely

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

var erjNonceRE = regexp.MustCompile(`var\s+ERJ\s*=\s*\{[\s\S]*?nonce\s*:\s*'([^']+)'`)

// DiscoverNonce loads the public homepage and extracts the ERJ AJAX nonce (inline script).
func DiscoverNonce(ctx context.Context, client *http.Client) (string, error) {
	return DiscoverNonceFromURL(ctx, client, DefaultSiteBaseURL)
}

// DiscoverNonceFromURL is like DiscoverNonce but uses an explicit homepage URL (e.g. httptest server).
func DiscoverNonceFromURL(ctx context.Context, client *http.Client, homeURL string) (string, error) {
	if client == nil {
		client = utils.NewHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, homeURL, http.NoBody)
	if err != nil {
		return "", err
	}
	utils.SetCollectorUserAgent(req)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("europe remotely: homepage: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("europe remotely: homepage: HTTP %s", resp.Status)
	}
	m := erjNonceRE.FindSubmatch(b)
	if len(m) < 2 {
		return "", fmt.Errorf("europe remotely: nonce not found in homepage")
	}
	return strings.TrimSpace(string(m[1])), nil
}
