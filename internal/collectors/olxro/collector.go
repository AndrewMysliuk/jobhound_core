// Package olxro implements the olx.ro keyword collector.
// Documents are loaded with rod. window.__PRERENDERED_STATE__ is read from the raw
// response because hydration removes it.
package olxro

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
const SourceName = "olx_ro"

const siteOrigin = "https://www.olx.ro"

// Olx fetches one keyword list and the oferta detail page for each organic row.
type Olx struct {
	Fetcher browserfetch.RawFetcher
}

// Name implements collectors.Collector.
func (*Olx) Name() string { return SourceName }

// Fetch implements collectors.Collector.
func (c *Olx) Fetch(ctx context.Context) ([]schema.Job, error) {
	return c.FetchWithQuery(ctx, "")
}

// FetchWithQuery implements collectors.QueryFetcher.
// One query loads /locuri-de-munca/q-{words}/ only.
func (c *Olx) FetchWithQuery(ctx context.Context, query string) ([]schema.Job, error) {
	if _, ok := listURL(query, 1); !ok {
		return nil, nil
	}
	if c == nil || c.Fetcher == nil {
		return nil, fmt.Errorf("olx_ro: browser fetcher is nil")
	}
	seen := make(map[string]struct{})
	var out []schema.Job
	for page := 1; ; page++ {
		rawURL, _ := listURL(query, page)
		doc, err := c.fetch(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		if !sameDocument(rawURL, doc.FinalURL) {
			return out, nil
		}
		list, err := parseListHTML(doc.HTML)
		if err != nil {
			return nil, err
		}
		if len(list.Ads) == 0 {
			return out, nil
		}
		jobs, err := c.jobsFromAds(ctx, list.Ads, seen)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
		if list.TotalPages > 0 && page >= list.TotalPages {
			return out, nil
		}
	}
}

func listURL(query string, page int) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(query))
	if len(fields) == 0 {
		return "", false
	}
	slug := url.PathEscape(strings.ToLower(strings.Join(fields, "-")))
	raw := siteOrigin + "/locuri-de-munca/q-" + slug + "/"
	if page > 1 {
		raw += "?page=" + strconv.Itoa(page)
	}
	return raw, true
}

func (c *Olx) jobsFromAds(ctx context.Context, ads []listAd, seen map[string]struct{}) ([]schema.Job, error) {
	var out []schema.Job
	for _, ad := range ads {
		if !strings.EqualFold(ad.SearchReason, "organic") {
			continue
		}
		if ad.ID == "" || ad.URL == "" {
			continue
		}
		if _, dup := seen[ad.ID]; dup {
			continue
		}
		seen[ad.ID] = struct{}{}
		doc, err := c.fetch(ctx, ad.URL)
		if err != nil {
			return nil, err
		}
		if !sameDocument(ad.URL, doc.FinalURL) {
			return nil, fmt.Errorf("olx_ro: GET %s: redirected to %s", ad.URL, doc.FinalURL)
		}
		job, err := parseDetailHTML(doc.HTML, ad)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

func (c *Olx) fetch(ctx context.Context, rawURL string) (browserfetch.NavigatedDocument, error) {
	if err := refuse(rawURL); err != nil {
		return browserfetch.NavigatedDocument{}, err
	}
	doc, err := c.Fetcher.FetchRaw(ctx, rawURL)
	if err != nil {
		return browserfetch.NavigatedDocument{}, fmt.Errorf("olx_ro: GET %s: %w", rawURL, err)
	}
	if strings.TrimSpace(doc.FinalURL) == "" {
		return browserfetch.NavigatedDocument{}, fmt.Errorf("olx_ro: GET %s: empty final URL", rawURL)
	}
	return doc, nil
}

func refuse(raw string) error {
	if strings.Contains(strings.ToLower(raw), "/api/") {
		return fmt.Errorf("olx_ro: refused %s", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("olx_ro: url: %w", err)
	}
	if strings.Contains(u.Path, "/d/oferta/") {
		return fmt.Errorf("olx_ro: refused %s", raw)
	}
	return nil
}

func sameDocument(requested, final string) bool {
	a, errA := url.Parse(strings.TrimSpace(requested))
	b, errB := url.Parse(strings.TrimSpace(final))
	if errA != nil || errB != nil || a.Host == "" || b.Host == "" {
		return false
	}
	return hostKey(a.Host) == hostKey(b.Host) && pathKey(a.Path) == pathKey(b.Path) && a.Query().Get("page") == b.Query().Get("page")
}

func hostKey(host string) string {
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}

func pathKey(path string) string {
	return strings.TrimRight(path, "/")
}
