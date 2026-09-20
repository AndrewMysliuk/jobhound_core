package vuejobs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var nuxtDataRE = regexp.MustCompile(`(?s)<script[^>]*id="__NUXT_DATA__"[^>]*>(.*?)</script>`)

type payload []any

// ListingPager is pagination metadata from the Nuxt jobs index payload.
type ListingPager struct {
	CurrentPage int
	LastPage    int
	PerPage     int
	Total       int
}

// ListingJob is one job row from __NUXT_DATA__ (not mapped to domain.Job).
type ListingJob struct {
	ID          string
	Slug        string
	Title       string
	Description string
	ApplyURL    string
	SourceURL   string
	Company     string
	PublishedAt string
	WorkPlace   []string
	PROLocked   bool
}

// ParseListingHTML extracts job rows and pager from a VueJobs SSR listing page.
func ParseListingHTML(html []byte) (jobs []ListingJob, pager ListingPager, err error) {
	raw, err := extractNUXTJSON(html)
	if err != nil {
		return nil, ListingPager{}, err
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, ListingPager{}, fmt.Errorf("vuejobs: decode __NUXT_DATA__: %w", err)
	}
	return extractListingFromPayload(p)
}

func extractNUXTJSON(html []byte) ([]byte, error) {
	m := nuxtDataRE.FindSubmatch(html)
	if len(m) < 2 {
		return nil, fmt.Errorf("vuejobs: __NUXT_DATA__ script not found")
	}
	return m[1], nil
}

func extractListingFromPayload(p payload) ([]ListingJob, ListingPager, error) {
	var jobs []ListingJob
	var pager ListingPager
	foundPager := false

	for _, slot := range p {
		m, ok := slot.(map[string]any)
		if !ok {
			continue
		}
		metaRef, hasMeta := m["meta"]
		dataRef, hasData := m["data"]
		if !hasMeta || !hasData {
			continue
		}
		meta, ok := p.resolve(metaRef, nil).(map[string]any)
		if !ok || meta["current_page"] == nil || meta["last_page"] == nil {
			continue
		}
		pager = ListingPager{
			CurrentPage: intFromAny(meta["current_page"]),
			LastPage:    intFromAny(meta["last_page"]),
			PerPage:     intFromAny(meta["per_page"]),
			Total:       intFromAny(meta["total"]),
		}
		foundPager = true

		dataList, ok := p.resolve(dataRef, nil).([]any)
		if !ok {
			continue
		}
		for _, itemRef := range dataList {
			row, ok := p.resolve(itemRef, nil).(map[string]any)
			if !ok || !isJobRowShape(row) {
				continue
			}
			j, ok := listingJobFromRow(p, row)
			if ok {
				jobs = append(jobs, j)
			}
		}
		break
	}
	if !foundPager {
		return nil, ListingPager{}, fmt.Errorf("vuejobs: listing pager not found in payload")
	}
	if len(jobs) == 0 && pager.Total > 0 && pager.CurrentPage <= pager.LastPage {
		return nil, pager, fmt.Errorf("vuejobs: no jobs extracted from payload")
	}
	return jobs, pager, nil
}

func isJobRowShape(m map[string]any) bool {
	_, hasID := m["id"]
	_, hasSlug := m["slug"]
	_, hasTitle := m["title"]
	return hasID && hasSlug && hasTitle
}

func listingJobFromRow(p payload, row map[string]any) (ListingJob, bool) {
	orgRef, hasOrg := row["organization"]
	if !hasOrg || orgRef == nil {
		return ListingJob{PROLocked: true}, false
	}
	org, ok := p.resolve(orgRef, nil).(map[string]any)
	if !ok {
		return ListingJob{PROLocked: true}, false
	}
	company := strings.TrimSpace(stringFromAny(org["name"]))
	if company == "" {
		return ListingJob{PROLocked: true}, false
	}

	slug := strings.TrimSpace(stringFromAny(p.resolve(row["slug"], nil)))
	title := strings.TrimSpace(stringFromAny(p.resolve(row["title"], nil)))
	if slug == "" || title == "" {
		return ListingJob{}, false
	}

	desc := strings.TrimSpace(stringFromAny(p.resolve(row["description"], nil)))
	apply := strings.TrimSpace(stringFromAny(p.resolve(row["apply_url"], nil)))
	source := strings.TrimSpace(stringFromAny(p.resolve(row["source_url"], nil)))
	published := strings.TrimSpace(stringFromAny(p.resolve(row["published_at"], nil)))

	var workPlace []string
	if wp, ok := p.resolve(row["work_place"], nil).([]any); ok {
		for _, w := range wp {
			if s := strings.TrimSpace(stringFromAny(w)); s != "" {
				workPlace = append(workPlace, s)
			}
		}
	}

	return ListingJob{
		ID:          stringFromAny(p.resolve(row["id"], nil)),
		Slug:        slug,
		Title:       title,
		Description: desc,
		ApplyURL:    apply,
		SourceURL:   source,
		Company:     company,
		PublishedAt: published,
		WorkPlace:   workPlace,
	}, true
}

func (p payload) resolve(v any, seen map[int]struct{}) any {
	if seen == nil {
		seen = make(map[int]struct{})
	}
	idx, ok := indexFromRef(v)
	if !ok {
		return v
	}
	if idx < 0 || idx >= len(p) {
		return v
	}
	if _, loop := seen[idx]; loop {
		return nil
	}
	seen[idx] = struct{}{}
	defer delete(seen, idx)

	slot := p[idx]
	switch t := slot.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = p.resolve(val, seen)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = p.resolve(val, seen)
		}
		return out
	default:
		return slot
	}
}

func indexFromRef(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		i := int(x)
		if float64(i) != x {
			return 0, false
		}
		return i, true
	case int:
		return x, true
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	default:
		return 0
	}
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}
