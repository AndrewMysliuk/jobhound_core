// Package himalayas implements the Himalayas public JSON jobs collector (specs/005-job-collectors/resources/himalayas.md).
package himalayas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value (contracts/collector.md).
const SourceName = "himalayas"

// DefaultBrowseURL is the full-feed JSON endpoint (resources/himalayas.md).
const DefaultBrowseURL = "https://himalayas.app/jobs/api"

// DefaultSearchURL is the filtered search JSON endpoint.
const DefaultSearchURL = "https://himalayas.app/jobs/api/search"

// DefaultMaxPages caps browse/search API rounds when MaxPages is 0 (worker/bootstrap).
const DefaultMaxPages = 5

// DefaultPageLimit is the server-enforced maximum page size for browse.
const DefaultPageLimit = 20

// Himalayas fetches jobs via GET JSON only (Tier 2).
type Himalayas struct {
	HTTPClient *http.Client
	Countries  *utils.CountryResolver
	// BrowseURL defaults to DefaultBrowseURL; SearchURL defaults to DefaultSearchURL.
	BrowseURL string
	SearchURL string
	// UseSearch selects /jobs/api/search instead of browse when true.
	UseSearch bool
	// SearchQuery is the free-text q parameter for search mode.
	SearchQuery string
	// SearchStartPage is 1-based; values < 1 are treated as 1.
	SearchStartPage int
	// MaxPages: 0 → DefaultMaxPages; -1 → no page cap; >0 → explicit cap.
	MaxPages int
	// MaxFetchJobs stops after this many jobs (0 = unlimited).
	MaxFetchJobs  int
	OnPubDateWarn func(raw float64)
}

// Name implements collectors.Collector.
func (*Himalayas) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Himalayas) Fetch(ctx context.Context) ([]schema.Job, error) {
	client := c.HTTPClient
	if client == nil {
		client = utils.NewHTTPClient()
	}
	maxPages := c.MaxPages
	switch {
	case maxPages == 0:
		maxPages = DefaultMaxPages
	case maxPages < 0:
		maxPages = 0
	}

	if c.UseSearch {
		return c.fetchSearchMode(ctx, client, maxPages)
	}
	return c.fetchBrowseMode(ctx, client, maxPages)
}

// FetchWithSlotSearch implements collectors.SlotSearchFetcher (Himalayas /jobs/api/search q=).
func (c *Himalayas) FetchWithSlotSearch(ctx context.Context, slotQuery string) ([]schema.Job, error) {
	q := strings.TrimSpace(slotQuery)
	if q == "" {
		return c.Fetch(ctx)
	}
	c2 := *c
	c2.UseSearch = true
	c2.SearchQuery = q
	if c2.SearchStartPage < 1 {
		c2.SearchStartPage = 1
	}
	return c2.Fetch(ctx)
}

func (c *Himalayas) fetchBrowseMode(ctx context.Context, client *http.Client, maxPages int) ([]schema.Job, error) {
	base := strings.TrimSpace(c.BrowseURL)
	if base == "" {
		base = DefaultBrowseURL
	}
	var all []schema.Job
	offset := 0
	limit := DefaultPageLimit
	pages := 0
	for {
		if maxPages > 0 && pages >= maxPages {
			break
		}
		u, err := url.Parse(base)
		if err != nil {
			return nil, fmt.Errorf("himalayas browse url: %w", err)
		}
		q := u.Query()
		q.Set("offset", strconv.Itoa(offset))
		q.Set("limit", strconv.Itoa(limit))
		u.RawQuery = q.Encode()

		env, err := c.getEnvelope(ctx, client, u.String())
		if err != nil {
			return nil, err
		}
		pages++

		for _, jw := range env.Jobs {
			j, err := jobFromWire(c.Countries, jw, c.OnPubDateWarn)
			if err != nil {
				return nil, err
			}
			all = append(all, j)
			if c.MaxFetchJobs > 0 && len(all) >= c.MaxFetchJobs {
				return all, nil
			}
		}

		if len(env.Jobs) == 0 {
			break
		}
		if len(env.Jobs) < limit {
			break
		}
		nextOff := offset + len(env.Jobs)
		if env.TotalCount > 0 && nextOff >= env.TotalCount {
			break
		}
		offset = nextOff
	}
	return all, nil
}

func (c *Himalayas) fetchSearchMode(ctx context.Context, client *http.Client, maxPages int) ([]schema.Job, error) {
	base := strings.TrimSpace(c.SearchURL)
	if base == "" {
		base = DefaultSearchURL
	}
	page := c.SearchStartPage
	if page < 1 {
		page = 1
	}
	q := strings.TrimSpace(c.SearchQuery)

	var all []schema.Job
	pages := 0
	for {
		if maxPages > 0 && pages >= maxPages {
			break
		}
		u, err := url.Parse(base)
		if err != nil {
			return nil, fmt.Errorf("himalayas search url: %w", err)
		}
		qs := u.Query()
		qs.Set("page", strconv.Itoa(page))
		if q != "" {
			qs.Set("q", q)
		}
		u.RawQuery = qs.Encode()

		env, err := c.getEnvelope(ctx, client, u.String())
		if err != nil {
			return nil, err
		}
		pages++

		if len(env.Jobs) == 0 {
			break
		}

		for _, jw := range env.Jobs {
			j, err := jobFromWire(c.Countries, jw, c.OnPubDateWarn)
			if err != nil {
				return nil, err
			}
			all = append(all, j)
			if c.MaxFetchJobs > 0 && len(all) >= c.MaxFetchJobs {
				return all, nil
			}
		}

		page++
		if env.TotalCount > 0 && env.Limit > 0 {
			totalPages := (env.TotalCount + env.Limit - 1) / env.Limit
			if page > totalPages {
				break
			}
		}
	}
	return all, nil
}

func (c *Himalayas) getEnvelope(ctx context.Context, client *http.Client, rawURL string) (apiEnvelope, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return apiEnvelope{}, err
	}
	req.Header.Set("Accept", "application/json")
	utils.SetCollectorUserAgent(req)

	resp, err := client.Do(req)
	if err != nil {
		return apiEnvelope{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiEnvelope{}, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return apiEnvelope{}, fmt.Errorf("himalayas: HTTP 429 Too Many Requests; backoff per operator guidance (https://himalayas.app/api)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return apiEnvelope{}, fmt.Errorf("himalayas: HTTP %s: %s", resp.Status, strings.TrimSpace(snippet))
	}
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return apiEnvelope{}, fmt.Errorf("himalayas: decode envelope: %w", err)
	}
	if env.Limit <= 0 && len(env.Jobs) > 0 {
		env.Limit = DefaultPageLimit
	}
	return env, nil
}
