// Package bdla implements the bdla.de job catalog collector.
package bdla

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
const SourceName = "bdla"

const (
	siteOrigin   = "https://www.bdla.de"
	listPath     = "/de/jobs-praktika/jobangebote"
	maxListPages = 40
)

// Bdla fetches the unfiltered job catalog with net/http.
type Bdla struct {
	HTTPClient *http.Client
}

// Name implements collectors.Collector.
func (*Bdla) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Bdla) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// The profile child is catalog, which ingest sends as an empty query.
// The list is view=ccbk, then page=2. The query is not sent. bl and toj are never sent.
func (c *Bdla) FetchWithQuery(ctx context.Context, _ string) ([]schema.Job, error) {
	client := c.httpClient()
	seen := make(map[string]struct{})
	var out []schema.Job
	for page := 1; page <= maxListPages; page++ {
		body, err := get(ctx, client, listURL(page))
		if err != nil {
			return nil, err
		}
		n, cards, err := parseListHTML(body)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			break
		}
		jobs, err := c.jobsFromCards(ctx, client, cards, seen)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
	}
	return out, nil
}

func listURL(page int) string {
	raw := siteOrigin + listPath + "?view=ccbk"
	if page > 1 {
		raw += "&page=" + strconv.Itoa(page)
	}
	return raw
}

func detailURL(id string) string {
	return siteOrigin + listPath + "?details=" + url.QueryEscape(id)
}

func (c *Bdla) jobsFromCards(ctx context.Context, client *http.Client, cards []listCard, seen map[string]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, card := range cards {
		if card.ID == "" {
			continue
		}
		if _, dup := seen[card.ID]; dup {
			continue
		}
		seen[card.ID] = struct{}{}
		body, err := get(ctx, client, card.URL)
		if err != nil {
			return nil, err
		}
		job, err := parseDetailHTML(body, card.URL)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func (c *Bdla) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return utils.NewHTTPClient()
}

func get(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if err := refuseFilters(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", utils.BrowserUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bdla: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bdla: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bdla: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}

func refuseFilters(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	for key := range u.Query() {
		switch strings.ToLower(key) {
		case "bl", "toj":
			return fmt.Errorf("bdla: refused %s", rawURL)
		}
	}
	return nil
}
