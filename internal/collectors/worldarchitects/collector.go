// Package worldarchitects implements the world-architects.com job catalog collector.
// Documents are loaded with rod. curl receives Cloudflare 403.
package worldarchitects

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/browserfetch"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// SourceName is the normative Job.Source value.
const SourceName = "world_architects"

const (
	siteOrigin   = "https://world-architects.com"
	listPath     = "/en/jobs"
	maxListPages = 40
)

// WorldArchitects fetches the unfiltered job catalog with rod.
type WorldArchitects struct {
	Fetcher browserfetch.NavigatedFetcher
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
	if c == nil || c.Fetcher == nil {
		return nil, fmt.Errorf("worldarchitects: browser fetcher is nil")
	}
	seen := make(map[string]struct{})
	var prev map[string]struct{}
	var out []schema.Job
	rejectDays := scoringschema.PostedAtRejectDaysArchitecture
	now := time.Now().UTC()
	for page := 1; page <= maxListPages; page++ {
		body, err := c.get(ctx, listURL(page))
		if err != nil {
			return nil, err
		}
		cards, err := parseListHTML(body)
		if err != nil {
			return nil, err
		}
		if listPageTooOldByPostedAt(cards, now, rejectDays) {
			break
		}
		slugs := slugSet(cards)
		if prev != nil && sameSlugs(prev, slugs) {
			break
		}
		jobs, err := c.jobsFromCards(ctx, cards, seen, now, rejectDays)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
		prev = slugs
	}
	return out, nil
}

func listPageTooOldByPostedAt(cards []listCard, now time.Time, rejectDays int) bool {
	if len(cards) == 0 {
		return false
	}
	var newest time.Time
	for _, card := range cards {
		if card.PostedAt.After(newest) {
			newest = card.PostedAt
		}
	}
	if newest.IsZero() {
		return false
	}
	return utils.PostedBeforeCutoff(newest, now, rejectDays)
}

func listURL(page int) string {
	if page <= 1 {
		return siteOrigin + listPath
	}
	return siteOrigin + listPath + "?page=" + strconv.Itoa(page)
}

func (c *WorldArchitects) jobsFromCards(ctx context.Context, cards []listCard, seen map[string]struct{}, now time.Time, rejectDays int) ([]schema.Job, error) {
	var out []schema.Job
	for _, card := range cards {
		if card.Slug == "" {
			continue
		}
		if utils.PostedBeforeCutoff(card.PostedAt, now, rejectDays) {
			continue
		}
		if _, dup := seen[card.Slug]; dup {
			continue
		}
		seen[card.Slug] = struct{}{}
		body, err := c.get(ctx, card.URL)
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

func (c *WorldArchitects) get(ctx context.Context, rawURL string) ([]byte, error) {
	if err := refuseTerm(rawURL); err != nil {
		return nil, err
	}
	doc, err := c.Fetcher.FetchNavigated(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("worldarchitects: GET %s: %w", rawURL, err)
	}
	return doc.HTML, nil
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
