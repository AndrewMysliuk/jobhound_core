// Package baunetz implements the baunetz.de stellenmarkt catalog collector.
package baunetz

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "baunetz"

const (
	siteOrigin = "https://www.baunetz.de"
	listURL    = siteOrigin + "/stellenmarkt/"
)

// Baunetz fetches the allowed /stellenmarkt/ page with net/http.
type Baunetz struct {
	HTTPClient *http.Client
}

// Name implements collectors.Collector.
func (*Baunetz) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Baunetz) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// The profile child is catalog, which ingest sends as an empty query.
// The list is the one allowed page: the query is not sent, and the page is not paginated.
func (c *Baunetz) FetchWithQuery(ctx context.Context, _ string) ([]schema.Job, error) {
	client := c.httpClient()
	body, err := get(ctx, client, listURL)
	if err != nil {
		return nil, err
	}
	links, err := parseListHTML(body)
	if err != nil {
		return nil, err
	}
	return c.jobsFromLinks(ctx, client, links)
}

func (c *Baunetz) jobsFromLinks(ctx context.Context, client *http.Client, links []string) ([]schema.Job, error) {
	seen := make(map[string]struct{})
	var out []schema.Job
	for _, link := range links {
		if _, dup := seen[link]; dup {
			continue
		}
		seen[link] = struct{}{}
		body, err := get(ctx, client, link)
		if err != nil {
			return nil, err
		}
		job, err := parseDetail(body, link)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func (c *Baunetz) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func get(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if err := refuseSearch(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", utils.BrowserUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("baunetz: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("baunetz: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("baunetz: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}

func refuseSearch(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	p := strings.ToLower(u.Path)
	if len(p) > 1 {
		p = strings.TrimSuffix(p, "/")
	}
	if p == "/suche" || p == "/stellenmarkt/suche" {
		return fmt.Errorf("baunetz: refused %s", rawURL)
	}
	return nil
}
