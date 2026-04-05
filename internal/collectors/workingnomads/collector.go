// Package workingnomads implements the Working Nomads board collector (specs/005-job-collectors).
package workingnomads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value (contracts/collector.md).
const SourceName = "working_nomads"

// DefaultSearchURL is the Elasticsearch-style jobs API (resources/working-nomads.md).
const DefaultSearchURL = "https://www.workingnomads.com/jobsapi/_search"

var (
	defaultWNQuery  = json.RawMessage(`{"match_all":{}}`)
	defaultWNSort   = json.RawMessage(`[{"premium":{"order":"desc"}},{"_score":{"order":"desc"}},{"pub_date":{"order":"desc"}}]`)
	defaultWNSource = []string{
		"id", "title", "slug", "company", "category_name", "description", "position_type",
		"tags", "all_tags", "locations", "location_base", "location_extra", "pub_date",
		"apply_option", "apply_email", "apply_url", "expired", "salary_range", "salary_range_short",
		"annual_salary_usd", "experience_level", "premium", "premium_subscription",
	}
)

type esSearchRequest struct {
	TrackTotalHits bool            `json:"track_total_hits"`
	From           int             `json:"from"`
	Size           int             `json:"size"`
	Query          json.RawMessage `json:"query"`
	Sort           json.RawMessage `json:"sort,omitempty"`
	Source         []string        `json:"_source"`
}

// WorkingNomads fetches jobs via POST jobsapi/_search (JSON only; no HTML detail fetch).
type WorkingNomads struct {
	HTTPClient *http.Client
	// SearchURL defaults to DefaultSearchURL.
	SearchURL string
	// PageSize defaults to 100.
	PageSize int
	// Query overrides the default match_all query (JSON object).
	Query json.RawMessage
	// Sort overrides the default sort array.
	Sort json.RawMessage
	// SourceFieldNames overrides returned _source field list; nil uses defaultWNSource.
	SourceFieldNames []string
	Countries        *utils.CountryResolver
	// MaxFetchJobs stops pagination after this many jobs are collected (0 = unlimited).
	// Debug HTTP and tests use this to avoid downloading the full index.
	MaxFetchJobs int
}

// Name implements collectors.Collector.
func (*WorkingNomads) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *WorkingNomads) Fetch(ctx context.Context) ([]schema.Job, error) {
	client := c.HTTPClient
	if client == nil {
		client = utils.NewHTTPClient()
	}
	rawURL := strings.TrimSpace(c.SearchURL)
	if rawURL == "" {
		rawURL = DefaultSearchURL
	}
	size := c.PageSize
	if size <= 0 {
		size = 100
	}
	query := c.Query
	if len(query) == 0 {
		query = defaultWNQuery
	}
	sort := c.Sort
	if len(sort) == 0 {
		sort = defaultWNSort
	}
	srcFields := c.SourceFieldNames
	if len(srcFields) == 0 {
		srcFields = defaultWNSource
	}

	var all []schema.Job
	from := 0
	for {
		reqBody := esSearchRequest{
			TrackTotalHits: true,
			From:           from,
			Size:           size,
			Query:          query,
			Sort:           sort,
			Source:         srcFields,
		}
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("working nomads: encode search: %w", err)
		}
		body, err := postJSON(ctx, client, rawURL, payload)
		if err != nil {
			return nil, fmt.Errorf("working nomads search from=%d: %w", from, err)
		}
		var resp searchResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("working nomads search from=%d: decode: %w", from, err)
		}
		hits := resp.Hits.Hits
		total := resp.Hits.Total.Value
		for _, h := range hits {
			j, err := jobFromSource(c.Countries, h.Source)
			if err != nil {
				if errors.Is(err, errSkipHit) {
					continue
				}
				return nil, err
			}
			all = append(all, j)
			if c.MaxFetchJobs > 0 && len(all) >= c.MaxFetchJobs {
				return all, nil
			}
		}
		from += len(hits)
		if from >= total || len(hits) == 0 {
			break
		}
	}
	return all, nil
}

func postJSON(ctx context.Context, client *http.Client, rawURL string, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	utils.SetCollectJSONPostHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(b)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(snippet))
	}
	return b, nil
}
