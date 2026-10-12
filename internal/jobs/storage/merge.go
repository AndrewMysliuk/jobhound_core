package storage

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// openingSpace matches the title identity rule: trim, collapse each whitespace run, lowercase.
var openingSpace = regexp.MustCompile(`\s+`)

func newIngestJob(incoming schema.Job, now time.Time) schema.Job {
	row := incoming
	row.Sources = mergeSourceIDs(nil, incoming.Source, incoming.Sources)
	row.FirstSeenAt = now
	row.LastSeenAt = now
	row.Openings = mergeOpenings(nil, openingFromListing(incoming))
	row.Location.Countries = unionSortedCodes(nil, incoming.Location.Countries)
	row.Location.Regions = unionSortedCodes(nil, incoming.Location.Regions)
	applyOpeningParent(&row)
	return row
}

func mergeIngestJob(existing, incoming schema.Job, now time.Time) schema.Job {
	out := existing
	out.Sources = mergeSourceIDs(existing.Sources, incoming.Source, incoming.Sources)
	out.LastSeenAt = now
	out.CompanyWebsite = fillCompanyWebsite(existing.CompanyWebsite, incoming.CompanyWebsite)
	out.PostedAt = laterPostedAt(existing.PostedAt, incoming.PostedAt)
	out.Openings = mergeOpenings(existing.Openings, openingFromListing(incoming))
	out.Location.Countries = unionSortedCodes(existing.Location.Countries, incoming.Location.Countries)
	out.Location.Regions = unionSortedCodes(existing.Location.Regions, incoming.Location.Regions)
	applyOpeningParent(&out)
	return out
}

func fillCompanyWebsite(existing, incoming string) string {
	if strings.TrimSpace(existing) == "" && strings.TrimSpace(incoming) != "" {
		return strings.TrimSpace(incoming)
	}
	return existing
}

func laterPostedAt(existing, incoming time.Time) time.Time {
	if incoming.IsZero() || (!existing.IsZero() && !incoming.After(existing)) {
		return existing
	}
	return incoming
}

func openingFromListing(job schema.Job) schema.Opening {
	return schema.Opening{
		Country:  strings.TrimSpace(job.Location.Raw),
		URL:      job.URL,
		ApplyURL: job.ApplyURL,
	}
}

type openingKey struct {
	country  string
	url      string
	applyURL string
}

func mergeOpenings(existing []schema.Opening, incoming schema.Opening) []schema.Opening {
	out := make([]schema.Opening, 0, len(existing)+1)
	seen := make(map[openingKey]struct{}, len(existing)+1)
	add := func(opening schema.Opening) {
		key := openingKey{
			country:  normalizeCountry(opening.Country),
			url:      opening.URL,
			applyURL: opening.ApplyURL,
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, opening)
	}
	for _, opening := range existing {
		add(opening)
	}
	add(incoming)
	sort.SliceStable(out, func(i, j int) bool {
		return openingLess(out[i], out[j])
	})
	return out
}

func openingLess(a, b schema.Opening) bool {
	ac := normalizeCountry(a.Country)
	bc := normalizeCountry(b.Country)
	if ac != bc {
		return ac < bc
	}
	if a.URL != b.URL {
		return a.URL < b.URL
	}
	return a.ApplyURL < b.ApplyURL
}

func normalizeCountry(s string) string {
	return strings.ToLower(openingSpace.ReplaceAllString(strings.TrimSpace(s), " "))
}

func applyOpeningParent(job *schema.Job) {
	job.Location.Raw = locationRaw(job.Openings)
	if len(job.Openings) == 0 {
		job.URL = ""
		job.ApplyURL = ""
		return
	}
	job.URL = job.Openings[0].URL
	job.ApplyURL = job.Openings[0].ApplyURL
}

func locationRaw(openings []schema.Opening) string {
	seen := make(map[string]struct{}, len(openings))
	labels := make([]countryLabel, 0, len(openings))
	for _, opening := range openings {
		text := strings.TrimSpace(opening.Country)
		norm := normalizeCountry(text)
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		labels = append(labels, countryLabel{norm: norm, text: text})
	}
	sort.SliceStable(labels, func(i, j int) bool {
		return labels[i].norm < labels[j].norm
	})
	parts := make([]string, len(labels))
	for i, label := range labels {
		parts[i] = label.text
	}
	return strings.Join(parts, ", ")
}

type countryLabel struct {
	norm string
	text string
}

func unionSortedCodes(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	out := make([]string, 0, len(existing)+len(incoming))
	add := func(codes []string) {
		for _, code := range codes {
			code = strings.TrimSpace(code)
			if code == "" {
				continue
			}
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			out = append(out, code)
		}
	}
	add(existing)
	add(incoming)
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}
