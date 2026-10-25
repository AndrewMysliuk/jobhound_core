// Package dou implements the DOU.ua jobs collector (RSS list + HTML detail).
package dou

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "dou_ua"

// DefaultFeedBase is the DOU RSS endpoint (query + remote appended by FeedURL).
const DefaultFeedBase = "https://jobs.dou.ua/vacancies/feeds/"

// Dou implements collectors.Collector + collectors.QueryFetcher.
type Dou struct {
	HTTPClient *http.Client
	// FeedBase defaults to DefaultFeedBase.
	FeedBase string
	// MaxJobs stops after this many mapped jobs (0 = unlimited). Debug/tests.
	MaxJobs int
}

// Name implements collectors.Collector.
func (*Dou) Name() string { return SourceName }

// Fetch implements collectors.Collector. DOU is query-driven — empty catalog.
func (d *Dou) Fetch(ctx context.Context) ([]schema.Job, error) {
	return d.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// Empty/whitespace query returns an empty list (not a catalog source).
func (d *Dou) FetchWithQuery(ctx context.Context, query string) ([]schema.Job, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	client := d.httpClient()
	feedURL := d.feedURL(query)
	body, err := fetchFeedBytes(ctx, client, feedURL)
	if err != nil {
		return nil, fmt.Errorf("dou_ua FETCH: %w", err)
	}
	items, err := feedItemsFromRSS(body)
	if err != nil {
		return nil, fmt.Errorf("dou_ua PARSE: %w", err)
	}

	var out []schema.Job
	for _, item := range items {
		j, ok := d.mapOne(ctx, client, item)
		if !ok {
			continue
		}
		out = append(out, j)
		if d.MaxJobs > 0 && len(out) >= d.MaxJobs {
			break
		}
	}
	return out, nil
}

func (d *Dou) mapOne(ctx context.Context, client *http.Client, item feedItem) (schema.Job, bool) {
	detailURL := strings.TrimSpace(item.Link)
	if detailURL == "" {
		detailURL = strings.TrimSpace(item.GUID)
	}

	var detail *detailFields
	if detailURL != "" {
		raw, err := fetchDetailBytes(ctx, client, detailURL)
		if err == nil {
			parsed, perr := parseDetailHTML(raw)
			if perr == nil {
				detail = &parsed
			}
		}
		// Detail transport/parse failure: soft-fill from RSS; do not fail the child.
	}

	j, err := jobFromFeedAndDetail(item, detail)
	if err != nil {
		return schema.Job{}, false
	}
	return j, true
}

func (d *Dou) httpClient() *http.Client {
	if d != nil && d.HTTPClient != nil {
		return d.HTTPClient
	}
	return utils.NewHTTPClient()
}
