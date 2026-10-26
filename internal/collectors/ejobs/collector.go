// Package ejobs implements the ejobs.ro keyword collector.
// List and detail documents are loaded with rod. curl receives Cloudflare 403.
package ejobs

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
const SourceName = "ejobs"

const (
	siteOrigin  = "https://www.ejobs.ro"
	maxListPage = 10
	pageSize    = 40
	requestGap  = 4 * time.Second
)

// Ejobs fetches the keyword list and the remote path for one query.
type Ejobs struct {
	Fetcher browserfetch.NavigatedFetcher
	// Gap is the wait between requests. Zero uses 4s. Negative skips the wait.
	Gap time.Duration
}

// Name implements collectors.Collector.
func (*Ejobs) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Ejobs) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// One query loads /locuri-de-munca/{words} and /locuri-de-munca/remote/{words}.
func (c *Ejobs) FetchWithQuery(ctx context.Context, query string) ([]schema.Job, error) {
	bases, ok := listingBases(query)
	if !ok {
		return nil, nil
	}
	if c == nil || c.Fetcher == nil {
		return nil, fmt.Errorf("ejobs: browser fetcher is nil")
	}
	seen := make(map[string]struct{})
	var out []schema.Job
	calls := 0
	for _, base := range bases {
		jobs, err := c.fetchBase(ctx, base, seen, &calls)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
	}
	return out, nil
}

func listingBases(query string) ([2]string, bool) {
	fields := strings.Fields(strings.TrimSpace(query))
	if len(fields) == 0 {
		return [2]string{}, false
	}
	slug := url.PathEscape(strings.ToLower(strings.Join(fields, "-")))
	return [2]string{
		siteOrigin + "/locuri-de-munca/" + slug,
		siteOrigin + "/locuri-de-munca/remote/" + slug,
	}, true
}

func pageURL(base string, page int) string {
	base = strings.TrimRight(base, "/")
	if page <= 1 {
		return base
	}
	return base + "/pagina" + strconv.Itoa(page)
}

func (c *Ejobs) fetchBase(ctx context.Context, base string, seen map[string]struct{}, calls *int) ([]schema.Job, error) {
	var out []schema.Job
	rejectDays := scoringschema.PostedAtRejectDays
	now := time.Now().UTC()
	for page := 1; page <= maxListPage; page++ {
		rawURL := pageURL(base, page)
		doc, err := c.fetch(ctx, rawURL, calls)
		if err != nil {
			return nil, err
		}
		if !samePage(rawURL, doc.FinalURL) {
			return out, nil
		}
		list, err := parseListHTML(doc.HTML)
		if err != nil {
			return nil, err
		}
		if list.FromList {
			if listPageTooOldByPostedAt(list.Jobs, now, rejectDays) {
				return out, nil
			}
			for _, job := range list.Jobs {
				if utils.PostedBeforeCutoff(job.PostedAt, now, rejectDays) {
					continue
				}
				if _, dup := seen[job.URL]; dup {
					continue
				}
				seen[job.URL] = struct{}{}
				out = append(out, job)
			}
		} else {
			jobs, err := c.jobsFromCards(ctx, list.Cards, seen, calls)
			if err != nil {
				return nil, err
			}
			out = append(out, jobs...)
		}
		if list.Count < pageSize {
			return out, nil
		}
	}
	return out, nil
}

func listPageTooOldByPostedAt(jobs []schema.Job, now time.Time, rejectDays int) bool {
	if len(jobs) == 0 {
		return false
	}
	var newest time.Time
	for _, job := range jobs {
		if job.PostedAt.After(newest) {
			newest = job.PostedAt
		}
	}
	if newest.IsZero() {
		return false
	}
	return utils.PostedBeforeCutoff(newest, now, rejectDays)
}

func (c *Ejobs) jobsFromCards(ctx context.Context, cards []listCard, seen map[string]struct{}, calls *int) ([]schema.Job, error) {
	var out []schema.Job
	for _, card := range cards {
		if _, dup := seen[card.URL]; dup {
			continue
		}
		seen[card.URL] = struct{}{}
		doc, err := c.fetch(ctx, card.URL, calls)
		if err != nil {
			return nil, err
		}
		if !samePage(card.URL, doc.FinalURL) {
			return nil, fmt.Errorf("ejobs: GET %s: redirected to %s", card.URL, doc.FinalURL)
		}
		job, err := parseDetailHTML(doc.HTML, card.URL, card.Title)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func (c *Ejobs) fetch(ctx context.Context, rawURL string, calls *int) (browserfetch.NavigatedDocument, error) {
	if err := c.pause(ctx, *calls); err != nil {
		return browserfetch.NavigatedDocument{}, err
	}
	*calls++
	doc, err := c.Fetcher.FetchNavigated(ctx, rawURL)
	if err != nil {
		return browserfetch.NavigatedDocument{}, fmt.Errorf("ejobs: GET %s: %w", rawURL, err)
	}
	if strings.TrimSpace(doc.FinalURL) == "" {
		return browserfetch.NavigatedDocument{}, fmt.Errorf("ejobs: GET %s: empty final URL", rawURL)
	}
	return doc, nil
}

func (c *Ejobs) pause(ctx context.Context, calls int) error {
	if calls == 0 {
		return nil
	}
	d := c.gap()
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *Ejobs) gap() time.Duration {
	if c != nil && c.Gap < 0 {
		return 0
	}
	if c != nil && c.Gap > requestGap {
		return c.Gap
	}
	return requestGap
}

func samePage(requested, final string) bool {
	a, errA := url.Parse(strings.TrimSpace(requested))
	b, errB := url.Parse(strings.TrimSpace(final))
	if errA != nil || errB != nil || a.Host == "" || b.Host == "" {
		return false
	}
	return hostKey(a.Host) == hostKey(b.Host) && pathKey(a.Path) == pathKey(b.Path)
}

func hostKey(host string) string {
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}

func pathKey(path string) string {
	return strings.TrimRight(path, "/")
}
