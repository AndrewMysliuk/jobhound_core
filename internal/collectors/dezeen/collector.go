// Package dezeen implements the dezeenjobs.com category collector.
package dezeen

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "dezeen_jobs"

const (
	siteOrigin   = "https://www.dezeenjobs.com"
	maxListPages = 40
)

// Dezeen fetches one job-category slug with net/http.
type Dezeen struct {
	HTTPClient *http.Client
}

// Name implements collectors.Collector.
func (*Dezeen) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Dezeen) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// query is one category slug. An empty slug fetches nothing.
func (c *Dezeen) FetchWithQuery(ctx context.Context, query string) ([]schema.Job, error) {
	slug, ok := categorySlug(query)
	if !ok {
		return nil, nil
	}
	client := c.httpClient()
	seen := make(map[string]struct{})
	var out []schema.Job
	for page := 1; page <= maxListPages; page++ {
		body, err := get(ctx, client, listURL(slug, page))
		if err != nil {
			return nil, err
		}
		rows, err := parseListHTML(body)
		if err != nil {
			return nil, err
		}
		// An empty .main-jobs-list ends the walk. rel=next stays on empty pages.
		if len(rows) == 0 {
			break
		}
		jobs, err := c.jobsFromRows(ctx, client, rows, seen)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
	}
	return out, nil
}

func categorySlug(query string) (string, bool) {
	slug := strings.Trim(strings.TrimSpace(query), "/")
	if slug == "" || strings.EqualFold(slug, "feed") {
		return "", false
	}
	if strings.ContainsAny(slug, `/\?#`) || strings.Contains(slug, "..") {
		return "", false
	}
	return slug, true
}

func listURL(slug string, page int) string {
	base := siteOrigin + "/job-category/" + url.PathEscape(slug) + "/"
	if page <= 1 {
		return base
	}
	return base + "page/" + strconv.Itoa(page) + "/"
}

func (c *Dezeen) jobsFromRows(ctx context.Context, client *http.Client, rows []listRow, seen map[string]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, row := range rows {
		if _, dup := seen[row.ID]; dup {
			continue
		}
		seen[row.ID] = struct{}{}
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

func (c *Dezeen) httpClient() *http.Client {
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
		return nil, fmt.Errorf("dezeen: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("dezeen: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dezeen: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}
