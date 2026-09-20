package weworkremotely

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

func fetchFeedBytes(ctx context.Context, client *http.Client, feedURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	utils.SetCollectorUserAgent(req)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(snippet))
	}
	return body, nil
}
