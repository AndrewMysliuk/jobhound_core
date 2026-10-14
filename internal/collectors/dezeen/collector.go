// Package dezeen implements the dezeenjobs.com category collector.
// Documents are loaded with rod. curl receives Cloudflare 403.
package dezeen

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/browserfetch"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "dezeen_jobs"

const (
	siteOrigin   = "https://www.dezeenjobs.com"
	maxListPages = 40
)

// Dezeen fetches one job-category slug with rod.
type Dezeen struct {
	Fetcher browserfetch.NavigatedFetcher
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
	if c == nil || c.Fetcher == nil {
		return nil, fmt.Errorf("dezeen: browser fetcher is nil")
	}
	seen := make(map[string]struct{})
	var out []schema.Job
	for page := 1; page <= maxListPages; page++ {
		body, err := c.get(ctx, listURL(slug, page))
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
		jobs, err := c.jobsFromRows(ctx, rows, seen)
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

func (c *Dezeen) jobsFromRows(ctx context.Context, rows []listRow, seen map[string]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, row := range rows {
		if _, dup := seen[row.ID]; dup {
			continue
		}
		seen[row.ID] = struct{}{}
		body, err := c.get(ctx, row.URL)
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

func (c *Dezeen) get(ctx context.Context, rawURL string) ([]byte, error) {
	doc, err := c.Fetcher.FetchNavigated(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("dezeen: GET %s: %w", rawURL, err)
	}
	return doc.HTML, nil
}
