// Package worldarchitects implements the world-architects.com job catalog collector.
package worldarchitects

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
const SourceName = "world_architects"

const (
	siteOrigin   = "https://world-architects.com"
	listPath     = "/en/jobs"
	maxListPages = 40
)

// WorldArchitects fetches the unfiltered job catalog with net/http.
type WorldArchitects struct {
	HTTPClient *http.Client
}

// Name implements collectors.Collector.
func (*WorldArchitects) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *WorldArchitects) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// The profile child is catalog, which ingest sends as an empty query.
// Pages are /en/jobs then ?page=2. The query is not sent. term is never sent.
// A repeated slug set is the last page; the footer pager is not.
func (c *WorldArchitects) FetchWithQuery(ctx context.Context, _ string) ([]schema.Job, error) {
	client := c.httpClient()
	seen := make(map[string]struct{})
	var prev map[string]struct{}
	var out []schema.Job
	for page := 1; page <= maxListPages; page++ {
		body, err := get(ctx, client, listURL(page))
		if err != nil {
			return nil, err
		}
		cards, err := parseListHTML(body)
		if err != nil {
			return nil, err
		}
		slugs := slugSet(cards)
		if prev != nil && sameSlugs(prev, slugs) {
			break
		}
		jobs, err := c.jobsFromCards(ctx, client, cards, seen)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
		prev = slugs
	}
	return out, nil
}

func listURL(page int) string {
	if page <= 1 {
		return siteOrigin + listPath
	}
	return siteOrigin + listPath + "?page=" + strconv.Itoa(page)
}

func (c *WorldArchitects) jobsFromCards(ctx context.Context, client *http.Client, cards []listCard, seen map[string]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, card := range cards {
		if card.Slug == "" {
			continue
		}
		if _, dup := seen[card.Slug]; dup {
			continue
		}
		seen[card.Slug] = struct{}{}
		body, err := get(ctx, client, card.URL)
		if err != nil {
			return nil, err
		}
		job, err := parseDetailHTML(body, card)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func slugSet(cards []listCard) map[string]struct{} {
	out := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		if card.Slug == "" {
			continue
		}
		out[card.Slug] = struct{}{}
	}
	return out
}

func sameSlugs(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for slug := range a {
		if _, ok := b[slug]; !ok {
			return false
		}
	}
	return true
}

// httpClient drops any cookie jar. The board stores the search filter in the session.
func (c *WorldArchitects) httpClient() *http.Client {
	base := utils.NewHTTPClient()
	if c != nil && c.HTTPClient != nil {
		base = c.HTTPClient
	}
	clone := *base
	clone.Jar = nil
	return &clone
}

func get(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if err := refuseTerm(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", utils.BrowserUserAgent)
	req.Header.Del("Cookie")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worldarchitects: GET %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("worldarchitects: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("worldarchitects: GET %s: HTTP %s", rawURL, resp.Status)
	}
	return body, nil
}

func refuseTerm(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	for key := range u.Query() {
		if strings.EqualFold(key, "term") {
			return fmt.Errorf("worldarchitects: refused %s", rawURL)
		}
	}
	return nil
}
