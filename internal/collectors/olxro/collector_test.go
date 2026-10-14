package olxro

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/browserfetch"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetchWithQuery_organicDetailAndPages(t *testing.T) {
	list1 := siteOrigin + "/locuri-de-munca/q-arhitect-proiectant/"
	list2 := list1 + "?page=2"
	list3 := list1 + "?page=3"
	remote := siteOrigin + "/oferta/loc-de-munca/arhitect-proiectant-cluj-IDabc.html"
	hybrid := siteOrigin + "/oferta/loc-de-munca/peisagist-IDhyb.html"
	open := siteOrigin + "/oferta/loc-de-munca/urbanist-IDopen.html"
	hidden := siteOrigin + "/oferta/loc-de-munca/confidential-IDhid.html"
	travel := siteOrigin + "/oferta/loc-de-munca/desenator-IDtrv.html"
	promoted := siteOrigin + "/oferta/loc-de-munca/promoted-IDpro.html"
	apiOffer := siteOrigin + "/api/v1/offers/9"
	deeplink := siteOrigin + "/d/oferta/arhitect-IDabc.html"

	pages := map[string]browserfetch.NavigatedDocument{
		list1: {FinalURL: list1, HTML: statePage(listState(3, []map[string]any{
			{
				"id": 101, "title": "Arhitect proiectant",
				"description":  "<p>Proiectare <b>cladiri</b></p>",
				"url":          deeplink,
				"urlPath":      "/oferta/loc-de-munca/arhitect-proiectant-cluj-IDabc.html",
				"searchReason": "organic",
				"createdTime":  "2026-10-01T08:00:00Z",
				"params": []any{
					map[string]any{"key": "job_character", "value": "can-work-remote"},
					map[string]any{"key": "salary", "value": map[string]any{
						"from": 5000, "to": 8000, "currencyCode": "RON", "gross": true,
					}},
				},
			},
			{
				"id": 900, "title": "Promoted", "searchReason": "promoted",
				"url": promoted,
			},
			{
				"id": 901, "title": "API", "searchReason": "organic",
				"url": apiOffer,
			},
			{
				"id": 902, "title": "Deeplink only", "searchReason": "organic",
				"url": deeplink,
			},
			{
				"id": 102, "title": "Peisagist", "description": "La birou",
				"url": hybrid + "?utm=1", "searchReason": "organic",
				"salary": "5000 RON",
				"params": []any{map[string]any{"key": "job_character", "normalizedValue": "hybrid"}},
			},
			{
				"id": 103, "title": "Urbanist", "searchReason": "organic",
				"urlPath": open, "job_character": "",
			},
		}), "")},
		list2: {FinalURL: list2, HTML: statePage(listState(3, []map[string]any{
			{
				"id": 101, "title": "Again", "searchReason": "organic",
				"url": remote,
			},
			{
				"id": 104, "title": "Arhitect", "searchReason": "organic",
				"url": hidden, "createdTime": "2026-10-02",
				"params": []any{map[string]any{"key": "job_character", "value": "stable-location-wanted"}},
			},
			{
				"id": 105, "title": "Desenator", "searchReason": "organic",
				"url":    travel,
				"params": []any{map[string]any{"key": "job_character", "value": "need-to-travel"}},
			},
		}), "")},
		list3:  {FinalURL: list3, HTML: statePage(listState(3, nil), "")},
		remote: {FinalURL: remote, HTML: detailPage("Studio", "confidential", "România", "2020-01-01")},
		hybrid: {FinalURL: hybrid, HTML: detailPage("Firm", "confidential", "RO", "2026-10-03T09:00:00Z")},
		open:   {FinalURL: open, HTML: detailPage("Atelier", "confidential", "", "")},
		hidden: {FinalURL: hidden, HTML: detailPage("Confidențial", "confidential", "RO", "")},
		travel: {FinalURL: travel, HTML: detailPage("Firm", "confidential", "RO", "")},
	}

	fake := &fakeFetcher{pages: pages}
	c := &Olx{Fetcher: fake}
	jobs, err := c.FetchWithQuery(context.Background(), "  Arhitect   proiectant ")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Equal(t, []string{list1, remote, hybrid, open, list2, hidden, travel, list3}, fake.urls)

	for _, raw := range fake.urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.NotContains(t, raw, "/api/")
		require.NotContains(t, u.Path, "/d/oferta/")
		require.NotContains(t, raw, "filter_enum")
		require.NotContains(t, raw, "can-work-remote")
		if u.RawQuery != "" {
			require.Equal(t, []string{"page"}, queryKeys(u))
		}
	}

	require.Len(t, jobs, 5)
	require.Equal(t, "Arhitect proiectant", jobs[0].Title)
	require.Equal(t, "Studio", jobs[0].Company)
	require.Equal(t, remote, jobs[0].URL)
	require.Equal(t, "Proiectare cladiri", jobs[0].Description)
	require.Equal(t, "5000-8000 RON", jobs[0].SalaryRaw)
	require.NotContains(t, jobs[0].SalaryRaw, "9999")
	require.Empty(t, jobs[0].ApplyURL)
	require.Equal(t, time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC), jobs[0].PostedAt)
	require.Equal(t, schema.LocationRemote, jobs[0].Location.Type)
	require.Equal(t, []string{"RO"}, jobs[0].Location.Countries)
	require.Equal(t, domainutils.StableJobID(jobs[0].CompanyKey, jobs[0].Title), jobs[0].ID)
	require.NotContains(t, jobs[0].ID, jobs[0].URL)

	require.Equal(t, "Peisagist", jobs[1].Title)
	require.Equal(t, hybrid, jobs[1].URL)
	require.Empty(t, jobs[1].SalaryRaw)
	require.Equal(t, schema.LocationHybrid, jobs[1].Location.Type)
	require.Equal(t, []string{"RO"}, jobs[1].Location.Countries)
	require.Equal(t, time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC), jobs[1].PostedAt)

	require.Equal(t, "Urbanist", jobs[2].Title)
	require.Equal(t, "Atelier", jobs[2].Company)
	require.Empty(t, jobs[2].Location.Type)
	require.Empty(t, jobs[2].Location.Countries)
	require.Empty(t, jobs[2].SalaryRaw)
	require.True(t, jobs[2].PostedAt.IsZero())

	require.Equal(t, "Arhitect", jobs[3].Title)
	require.Equal(t, hidden, jobs[3].URL)
	require.Contains(t, jobs[3].ID, jobs[3].URL)
	require.Equal(t, schema.LocationOffice, jobs[3].Location.Type)
	require.Equal(t, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), jobs[3].PostedAt)

	require.Equal(t, schema.LocationOffice, jobs[4].Location.Type)
	require.Equal(t, "Desenator", jobs[4].Title)
	for _, job := range jobs {
		require.NotEqual(t, "Promoted", job.Title)
		require.NotEqual(t, "API", job.Title)
		require.NotEqual(t, "Deeplink only", job.Title)
		require.NotEqual(t, "Again", job.Title)
		require.NotEqual(t, "confidential", job.Company)
	}
}

func TestFetchWithQuery_stopsAtTotalPages(t *testing.T) {
	list := siteOrigin + "/locuri-de-munca/q-urbanist/"
	listing := siteOrigin + "/oferta/loc-de-munca/urbanist-IDone.html"
	fake := &fakeFetcher{pages: map[string]browserfetch.NavigatedDocument{
		list: {FinalURL: list, HTML: statePage(listState(1, []map[string]any{{
			"id": 1, "title": "Urbanist", "searchReason": "organic", "url": listing,
		}}), "")},
		listing: {FinalURL: listing, HTML: detailPage("Firm", "confidential", "RO", "")},
	}}
	jobs, err := (&Olx{Fetcher: fake}).FetchWithQuery(context.Background(), "urbanist")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, []string{list, listing}, fake.urls)
}

func TestFetch_emptyQueryDoesNotFetch(t *testing.T) {
	fake := &fakeFetcher{}
	jobs, err := (&Olx{Fetcher: fake}).Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Empty(t, fake.urls)
}

func TestFetchWithQuery_nilFetcher(t *testing.T) {
	_, err := (&Olx{}).FetchWithQuery(context.Background(), "arhitect")
	require.Error(t, err)
}

func queryKeys(u *url.URL) []string {
	var keys []string
	for key := range u.Query() {
		keys = append(keys, key)
	}
	return keys
}

func listState(total int, ads []map[string]any) map[string]any {
	if ads == nil {
		ads = []map[string]any{}
	}
	return map[string]any{
		"listing": map[string]any{
			"listing": map[string]any{
				"ads":        ads,
				"totalPages": total,
			},
		},
	}
}

func statePage(state any, extra string) []byte {
	inner, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	quoted, err := json.Marshal(string(inner))
	if err != nil {
		panic(err)
	}
	return []byte(`<html><head><script>window.__PRERENDERED_STATE__ = ` + string(quoted) + `;</script></head><body>` + extra + `</body></html>`)
}

func detailPage(company, ldCompany, country, posted string) []byte {
	state := map[string]any{
		"hiringOrganization": map[string]any{"name": "not-from-state"},
		"ad": map[string]any{
			"ad": map[string]any{
				"postingComponent": map[string]any{"companyName": company},
			},
		},
	}
	payload := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "JobPosting",
		"title":       "Ignored",
		"description": "<p>ignored description</p>",
		"datePosted":  posted,
		"hiringOrganization": map[string]any{
			"@type": "Organization",
			"name":  ldCompany,
		},
		"baseSalary": map[string]any{"value": "9999"},
	}
	if country != "" {
		payload["jobLocation"] = map[string]any{
			"@type": "Place",
			"address": map[string]any{
				"@type":          "PostalAddress",
				"addressCountry": country,
			},
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return statePage(state, `<script type="application/ld+json">`+string(raw)+`</script>`)
}

type fakeFetcher struct {
	mu    sync.Mutex
	urls  []string
	pages map[string]browserfetch.NavigatedDocument
}

func (f *fakeFetcher) FetchRaw(_ context.Context, rawURL string) (browserfetch.NavigatedDocument, error) {
	f.mu.Lock()
	f.urls = append(f.urls, rawURL)
	f.mu.Unlock()
	doc, ok := f.pages[rawURL]
	if !ok {
		return browserfetch.NavigatedDocument{}, fmt.Errorf("unexpected %s", rawURL)
	}
	return doc, nil
}
