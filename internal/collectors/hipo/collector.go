// Package hipo implements the hipo.ro architecture-category collector.
package hipo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "hipo"

const (
	siteOrigin     = "https://www.hipo.ro"
	categoryPath   = "/locuri-de-munca/cautajob/Arhitectura-Design-Proiectare"
	categoryURL    = siteOrigin + categoryPath
	promoListingID = "103395"
	maxListPages   = 40
)

// Hipo fetches the architecture category and ignores the search query.
type Hipo struct {
	HTTPClient *http.Client
}

// Name implements collectors.Collector.
func (*Hipo) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Hipo) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// The query is ignored. Every call loads the architecture category.
func (c *Hipo) FetchWithQuery(ctx context.Context, _ string) ([]schema.Job, error) {
	client := c.httpClient()
	seen := make(map[string]struct{})
	var out []schema.Job
	last := 0
	for page := 1; page <= maxListPages; page++ {
		body, err := get(ctx, client, listURL(page))
		if err != nil {
			return nil, err
		}
		rows, pageLast, err := parseListHTML(body)
		if err != nil {
			return nil, err
		}
		if page == 1 {
			last = pageLast
		}
		if len(rows) == 0 {
			break
		}
		jobs, err := c.jobsFromRows(ctx, client, rows, seen)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
		if last >= 1 && page >= last {
			break
		}
	}
	return out, nil
}

func listURL(page int) string {
	if page <= 1 {
		return categoryURL
	}
	return categoryURL + "/" + strconv.Itoa(page)
}

func (c *Hipo) jobsFromRows(ctx context.Context, client *http.Client, rows []listRow, seen map[string]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, row := range rows {
		if row.ID == promoListingID {
			continue
		}
		if _, dup := seen[row.URL]; dup {
			continue
		}
		seen[row.URL] = struct{}{}
		body, err := get(ctx, client, row.URL)
		if err != nil {
			return nil, err
		}
		job, err := parseDetailHTML(body, row.URL)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func (c *Hipo) httpClient() *http.Client {
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
		return nil, fmt.Errorf("hipo: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hipo: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hipo: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}
