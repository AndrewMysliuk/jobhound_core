// Package getro implements the Getro portfolio board collector.
package getro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

const (
	defaultMaxPages = 25
	hitsPerPage     = 20
	searchURLFmt    = "https://api.getro.com/api/v2/collections/%d/search/jobs"
)

// Tenant is one Getro fund. CollectionID is the only tenant key.
type Tenant struct {
	SourceID     string
	CollectionID int
}

// Board fetches one Getro collection. The shared HTTP client is enough.
type Board struct {
	HTTPClient *http.Client
	Countries  *utils.CountryResolver
	Tenant     Tenant
	// MaxPages caps search requests. 0 means 25.
	MaxPages int
}

type searchRequest struct {
	HitsPerPage int           `json:"hitsPerPage"`
	Page        int           `json:"page"`
	Query       string        `json:"query"`
	Filters     searchFilters `json:"filters"`
}

type searchFilters struct {
	SearchableLocations      []string `json:"searchable_locations"`
	SearchableLocationOption []string `json:"searchable_location_option"`
}

// Tenants is every Getro fund whose collection id is known.
func Tenants() []Tenant {
	return []Tenant{
		{SourceID: "getro_accel", CollectionID: 8672},
		{SourceID: "getro_general_catalyst", CollectionID: 222},
		{SourceID: "getro_atomico", CollectionID: 36986},
		{SourceID: "getro_hv_capital", CollectionID: 234},
		{SourceID: "getro_point_nine", CollectionID: 1680},
		{SourceID: "getro_earlybird", CollectionID: 617},
		{SourceID: "getro_firstminute", CollectionID: 178},
		{SourceID: "getro_craft_ventures", CollectionID: 340},
	}
}

// Name implements collectors.Collector.
func (b *Board) Name() string {
	if b == nil {
		return ""
	}
	return b.Tenant.SourceID
}

// Fetch implements collectors.Collector. Getro has no catalog fetch.
func (b *Board) Fetch(context.Context) ([]schema.Job, error) {
	return nil, nil
}

// FetchWithQuery implements collectors.QueryFetcher.
func (b *Board) FetchWithQuery(ctx context.Context, query string) ([]schema.Job, error) {
	word, ok := searchWord(query)
	if !ok {
		return nil, nil
	}

	client := b.httpClient()
	seen := make(map[string]struct{})
	var out []schema.Job
	for page := 0; page < b.maxPages(); page++ {
		hits, err := b.postSearch(ctx, client, word, page)
		if err != nil {
			return nil, err
		}
		if len(hits) == 0 || pageURLsAlreadySeen(hits, seen) {
			break
		}
		rememberURLs(hits, seen)
		for _, hit := range hits {
			job, keep := mapHit(b.Tenant.SourceID, b.Countries, hit)
			if !keep {
				continue
			}
			out = append(out, job)
		}
	}
	logMapped(ctx, b.Name(), out)
	return out, nil
}

func (b *Board) maxPages() int {
	if b == nil || b.MaxPages <= 0 {
		return defaultMaxPages
	}
	return b.MaxPages
}

func (b *Board) httpClient() *http.Client {
	if b != nil && b.HTTPClient != nil {
		return b.HTTPClient
	}
	return utils.NewHTTPClient()
}

func searchWord(query string) (string, bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return "", false
	}
	if q == "go" {
		return "golang", true
	}
	return q, true
}

func (b *Board) postSearch(ctx context.Context, client *http.Client, query string, page int) ([]jobHit, error) {
	payload, err := json.Marshal(searchRequest{
		HitsPerPage: hitsPerPage,
		Page:        page,
		Query:       query,
		Filters: searchFilters{
			SearchableLocations:      []string{"Europe", "United States"},
			SearchableLocationOption: []string{"remote"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getro: encode search: %w", err)
	}
	endpoint := fmt.Sprintf(searchURLFmt, b.Tenant.CollectionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("getro: build search: %w", err)
	}
	// Getro returns 406 unless Accept is exactly application/json.
	req.Header.Set("Accept", "application/json")
	utils.SetCollectJSONPostHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getro: search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("getro: read search: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil, fmt.Errorf("getro: %w", collectors.ErrTenantUnavailable)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getro: HTTP %d", resp.StatusCode)
	}
	return decodeJobs(body)
}

func decodeJobs(body []byte) ([]jobHit, error) {
	var env struct {
		Results *struct {
			Jobs *[]jobHit `json:"jobs"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("getro: decode search: %w", err)
	}
	if env.Results == nil || env.Results.Jobs == nil {
		return nil, fmt.Errorf("getro: response missing results.jobs")
	}
	return *env.Results.Jobs, nil
}

func pageURLsAlreadySeen(hits []jobHit, seen map[string]struct{}) bool {
	sawURL := false
	for _, hit := range hits {
		u := strings.TrimSpace(hit.URL)
		if u == "" {
			continue
		}
		sawURL = true
		if _, ok := seen[u]; !ok {
			return false
		}
	}
	return sawURL
}

func rememberURLs(hits []jobHit, seen map[string]struct{}) {
	for _, hit := range hits {
		if u := strings.TrimSpace(hit.URL); u != "" {
			seen[u] = struct{}{}
		}
	}
}

func logMapped(ctx context.Context, sourceID string, jobs []schema.Job) {
	withApply := 0
	for _, job := range jobs {
		if strings.TrimSpace(job.ApplyURL) != "" {
			withApply++
		}
	}
	slog.InfoContext(ctx, "getro: fetch",
		"source", sourceID,
		"mapped", len(jobs),
		"apply_url", withApply,
	)
}
