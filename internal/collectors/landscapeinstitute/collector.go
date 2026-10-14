// Package landscapeinstitute implements the Landscape Institute jobs catalog collector.
package landscapeinstitute

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "landscape_institute"

const (
	siteOrigin = "https://jobs.landscapeinstitute.org"
	listURL    = siteOrigin + "/jobs/"
)

// LandscapeInstitute fetches the single /jobs/ catalog with net/http.
type LandscapeInstitute struct {
	HTTPClient *http.Client
}

// Name implements collectors.Collector.
func (*LandscapeInstitute) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *LandscapeInstitute) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// The profile child is catalog, which ingest sends as an empty query.
// search_keywords and search_region are not filters, so the query is not sent.
// /jobs/page/2/ repeats page 1 and is not fetched.
func (c *LandscapeInstitute) FetchWithQuery(ctx context.Context, _ string) ([]schema.Job, error) {
	client := c.httpClient()
	body, err := get(ctx, client, listURL)
	if err != nil {
		return nil, err
	}
	rows, err := parseListHTML(body)
	if err != nil {
		return nil, err
	}
	return c.jobsFromRows(ctx, client, rows)
}

func (c *LandscapeInstitute) jobsFromRows(ctx context.Context, client *http.Client, rows []listRow) ([]schema.Job, error) {
	seen := make(map[string]struct{})
	var out []schema.Job
	for _, row := range rows {
		if _, dup := seen[row.URL]; dup {
			continue
		}
		seen[row.URL] = struct{}{}
		body, err := get(ctx, client, row.URL)
		if err != nil {
			return nil, err
		}
		job, err := parseDetailHTML(body, row)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func (c *LandscapeInstitute) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func get(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", utils.BrowserUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("landscapeinstitute: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("landscapeinstitute: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("landscapeinstitute: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}
