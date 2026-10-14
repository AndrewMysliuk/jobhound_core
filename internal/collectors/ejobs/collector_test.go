package ejobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/browserfetch"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetchWithQuery_keywordRemoteAndRedirect(t *testing.T) {
	keyword := siteOrigin + "/locuri-de-munca/arhitect-proiectant"
	remote := siteOrigin + "/locuri-de-munca/remote/arhitect-proiectant"
	studio := siteOrigin + "/user/locuri-de-munca/arhitect-proiectant/200001"
	hidden := siteOrigin + "/user/locuri-de-munca/confidential/200002"
	office := siteOrigin + "/user/locuri-de-munca/peisagist/200003"
	poison := siteOrigin + "/user/locuri-de-munca/ignored/200099"

	pages := map[string]browserfetch.NavigatedDocument{
		keyword: {
			FinalURL: keyword,
			HTML: listHTML(40, []card{
				{url: studio, title: "List title"},
				{url: hidden + "?utm=1", title: "Arhitect"},
				{url: "https://example.com/not-ejobs", title: "Skip"},
			}, `<a href="`+keyword+`/pagina2?sort-publish=1">next</a><a href="`+keyword+`/pagina2?workModels=OFFICE">office</a>`),
		},
		keyword + "/pagina2": {
			FinalURL: keyword,
			HTML:     listHTML(40, []card{{url: poison, title: "Should not appear"}}, ""),
		},
		remote: {
			FinalURL: remote + "/",
			HTML:     listHTML(8, []card{{url: office, title: "Peisagist"}, {url: studio, title: "again"}}, ""),
		},
		studio: {FinalURL: studio, HTML: detailPage(detailFixture{
			title: "Arhitect proiectant", company: "Studio",
			description: "<p>Proiectare <b>cladiri</b></p>",
			posted:      "2026-10-01", city: "Cluj-Napoca", country: "RO",
			salary: "7000 - 9000 RON", work: "Hibrid",
		})},
		hidden: {FinalURL: hidden, HTML: detailPage(detailFixture{
			title: "Arhitect", company: "Confidențial",
			description: "<p>Remote studio</p>", posted: "2026-10-02T08:00:00Z",
			city: "București", country: "România", work: "Remote",
		})},
		office: {FinalURL: office, HTML: detailPage(detailFixture{
			title: "Peisagist", company: "Firm",
			description: "<p>La birou</p>", city: "Sibiu", country: "RO",
			work: "La birou/ on-site",
		})},
	}

	fake := &fakeFetcher{pages: pages}
	c := &Ejobs{Fetcher: fake, Gap: -1}
	jobs, err := c.FetchWithQuery(context.Background(), "  Arhitect   proiectant ")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Equal(t, []string{
		keyword,
		studio,
		hidden,
		keyword + "/pagina2",
		remote,
		office,
	}, fake.urls)

	for _, raw := range fake.urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.Empty(t, u.RawQuery)
		require.NotContains(t, raw, "pagina11")
		require.NotContains(t, raw, "pagina3")
		require.NotContains(t, raw, "sort-publish")
		require.NotContains(t, raw, "sort-geo")
		require.NotContains(t, raw, "workModels")
	}

	require.Len(t, jobs, 3)
	require.Equal(t, "Arhitect proiectant", jobs[0].Title)
	require.Equal(t, "Studio", jobs[0].Company)
	require.Equal(t, studio, jobs[0].URL)
	require.Equal(t, "Proiectare cladiri", jobs[0].Description)
	require.Equal(t, "7000 - 9000 RON", jobs[0].SalaryRaw)
	require.NotContains(t, jobs[0].SalaryRaw, "9999")
	require.Empty(t, jobs[0].ApplyURL)
	require.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), jobs[0].PostedAt)
	require.Equal(t, schema.LocationHybrid, jobs[0].Location.Type)
	require.Equal(t, []string{"RO"}, jobs[0].Location.Countries)
	require.Equal(t, "Cluj-Napoca", jobs[0].Location.Raw)
	require.Equal(t, domainutils.StableJobID(jobs[0].CompanyKey, jobs[0].Title), jobs[0].ID)
	require.NotContains(t, jobs[0].ID, jobs[0].URL)

	require.Equal(t, "Arhitect", jobs[1].Title)
	require.Equal(t, hidden, jobs[1].URL)
	require.Contains(t, jobs[1].ID, jobs[1].URL)
	require.Equal(t, schema.LocationRemote, jobs[1].Location.Type)
	require.Empty(t, jobs[1].SalaryRaw)
	require.Equal(t, []string{"RO"}, jobs[1].Location.Countries)
	require.False(t, jobs[1].PostedAt.IsZero())

	require.Equal(t, schema.LocationOffice, jobs[2].Location.Type)
	require.Equal(t, "Sibiu", jobs[2].Location.Raw)
	require.Empty(t, jobs[2].SalaryRaw)
	require.Empty(t, jobs[2].ApplyURL)
	for _, job := range jobs {
		require.NotEqual(t, "Should not appear", job.Title)
	}
}

func TestFetchWithQuery_stopsAtPagina10(t *testing.T) {
	fake := &seqFetcher{}
	c := &Ejobs{Fetcher: fake, Gap: -1}
	jobs, err := c.FetchWithQuery(context.Background(), "urbanist")
	require.NoError(t, err)
	require.Len(t, jobs, 20)

	var lists []string
	for _, raw := range fake.urls {
		require.NotContains(t, raw, "pagina11")
		require.NotContains(t, raw, "?")
		if strings.Contains(raw, "/user/") {
			continue
		}
		lists = append(lists, raw)
	}
	require.Len(t, lists, 20)
	require.Equal(t, siteOrigin+"/locuri-de-munca/urbanist/pagina10", lists[9])
	require.Equal(t, siteOrigin+"/locuri-de-munca/remote/urbanist/pagina10", lists[19])
}

func TestFetchWithQuery_readsNuxtList(t *testing.T) {
	keyword := siteOrigin + "/locuri-de-munca/arhitect"
	remote := siteOrigin + "/locuri-de-munca/remote/arhitect"
	studio := siteOrigin + "/user/locuri-de-munca/arhitect-stagiar/1990585"
	pages := map[string]browserfetch.NavigatedDocument{
		keyword: {
			FinalURL: keyword,
			HTML: nuxtListHTML([]nuxtJob{
				{id: 1990585, title: "Arhitect stagiar", slug: "arhitect-stagiar", company: "Studio", salary: "3000 - 5000 RON", address: "Sibiu, România", posted: "2026-10-13"},
				{id: 1990736, title: "ARHITECT", slug: "arhitect", company: "Firm", address: "București, România"},
			}, false),
		},
		remote: {FinalURL: remote, HTML: nuxtListHTML(nil, false)},
	}
	fake := &fakeFetcher{pages: pages}
	c := &Ejobs{Fetcher: fake, Gap: -1}
	jobs, err := c.FetchWithQuery(context.Background(), "arhitect")
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, []string{keyword, remote}, fake.urls)
	require.Equal(t, "Arhitect stagiar", jobs[0].Title)
	require.Equal(t, studio, jobs[0].URL)
	require.Equal(t, "Studio", jobs[0].Company)
	require.Equal(t, "3000 - 5000 RON", jobs[0].SalaryRaw)
	require.Equal(t, "Sibiu", jobs[0].Location.Raw)
	require.Equal(t, []string{"RO"}, jobs[0].Location.Countries)
	require.Equal(t, time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), jobs[0].PostedAt)
	require.Empty(t, jobs[0].Description)
	require.Equal(t, "ARHITECT", jobs[1].Title)
	require.Equal(t, "București", jobs[1].Location.Raw)
}

func TestFetchWithQuery_cloudflareBlock(t *testing.T) {
	keyword := siteOrigin + "/locuri-de-munca/arhitect"
	fake := &fakeFetcher{pages: map[string]browserfetch.NavigatedDocument{
		keyword: {FinalURL: keyword, HTML: []byte(`<html><head><title>Attention Required! | Cloudflare</title></head><body><div id="cf-error-details"><h1>Sorry, you have been blocked</h1></div></body></html>`)},
	}}
	_, err := (&Ejobs{Fetcher: fake, Gap: -1}).FetchWithQuery(context.Background(), "arhitect")
	require.ErrorContains(t, err, "cloudflare blocked")
}

func TestFetch_emptyQueryDoesNotFetch(t *testing.T) {
	fake := &fakeFetcher{}
	c := &Ejobs{Fetcher: fake, Gap: -1}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Empty(t, fake.urls)
}

func TestFetchWithQuery_nilFetcher(t *testing.T) {
	_, err := (&Ejobs{}).FetchWithQuery(context.Background(), "arhitect")
	require.Error(t, err)
}

func TestGap_atLeastFourSeconds(t *testing.T) {
	require.GreaterOrEqual(t, (&Ejobs{}).gap(), 4*time.Second)
	require.Equal(t, time.Duration(0), (&Ejobs{Gap: -1}).gap())
}

type card struct {
	url   string
	title string
}

type nuxtJob struct {
	id                      int64
	title, slug, company    string
	salary, address, posted string
}

func nuxtListHTML(items []nuxtJob, next bool) []byte {
	refs := make([]any, len(items))
	tail := []any{next}
	idx := 6
	for i, item := range items {
		refs[i] = idx
		tail = append(tail,
			map[string]any{
				"id": idx + 1, "title": idx + 2, "slug": idx + 3,
				"company": idx + 4, "salary": idx + 6, "locations": idx + 7, "creationDate": idx + 10,
			},
			item.id, item.title, item.slug,
			map[string]any{"name": idx + 5},
			item.company,
			item.salary,
			[]any{idx + 8},
			map[string]any{"address": idx + 9},
			item.address,
			item.posted,
		)
		idx += 11
	}
	data := []any{
		[]any{"ShallowReactive", 1},
		map[string]any{"pinia": 2},
		map[string]any{"jobs": 3},
		map[string]any{"_listItems": 4, "_nextBtnVisible": 5},
		refs,
	}
	data = append(data, tail...)
	raw, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return []byte(`<html><head><script type="application/json" id="__NUXT_DATA__">` + string(raw) + `</script></head><body></body></html>`)
}

func listHTML(count int, cards []card, extra string) []byte {
	items := make([]map[string]any, 0, len(cards))
	for i, card := range cards {
		items = append(items, map[string]any{
			"@type":    "ListItem",
			"position": i + 1,
			"name":     card.title,
			"url":      card.url,
		})
	}
	payload := map[string]any{
		"@context":        "https://schema.org",
		"@type":           "ItemList",
		"numberOfItems":   count,
		"itemListElement": items,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return []byte(`<html><body>` + extra + `<script type="application/ld+json">` + string(raw) + `</script></body></html>`)
}

type detailFixture struct {
	title       string
	company     string
	description string
	posted      string
	city        string
	country     string
	salary      string
	work        string
	extra       string
}

func detailPage(f detailFixture) []byte {
	payload := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "JobPosting",
		"title":       f.title,
		"description": f.description,
		"datePosted":  f.posted,
		"hiringOrganization": map[string]any{
			"@type": "Organization",
			"name":  f.company,
		},
		"jobLocation": map[string]any{
			"@type": "Place",
			"address": map[string]any{
				"@type":           "PostalAddress",
				"addressLocality": f.city,
				"addressCountry":  f.country,
			},
		},
		"baseSalary": map[string]any{"value": "9999"},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	var summary strings.Builder
	if f.salary != "" {
		summary.WriteString(`<div><span>Salariu</span><span>`)
		summary.WriteString(f.salary)
		summary.WriteString(`</span></div>`)
	}
	if f.work != "" {
		summary.WriteString(`<li><strong>Mod de lucru</strong> `)
		summary.WriteString(f.work)
		summary.WriteString(`</li>`)
	}
	return []byte(`<html><body><a href="https://www.ejobs.ro/apply/1">Aplică</a>` + f.extra +
		`<script type="application/ld+json">` + string(raw) + `</script>` + summary.String() + `</body></html>`)
}

type fakeFetcher struct {
	mu    sync.Mutex
	urls  []string
	pages map[string]browserfetch.NavigatedDocument
}

func (f *fakeFetcher) FetchNavigated(_ context.Context, rawURL string) (browserfetch.NavigatedDocument, error) {
	f.mu.Lock()
	f.urls = append(f.urls, rawURL)
	f.mu.Unlock()
	doc, ok := f.pages[rawURL]
	if !ok {
		return browserfetch.NavigatedDocument{}, fmt.Errorf("unexpected %s", rawURL)
	}
	return doc, nil
}

type seqFetcher struct {
	mu   sync.Mutex
	urls []string
}

func (f *seqFetcher) FetchNavigated(_ context.Context, rawURL string) (browserfetch.NavigatedDocument, error) {
	f.mu.Lock()
	f.urls = append(f.urls, rawURL)
	f.mu.Unlock()
	u, err := url.Parse(rawURL)
	if err != nil {
		return browserfetch.NavigatedDocument{}, err
	}
	if strings.Contains(u.Path, "/user/locuri-de-munca/") {
		id := u.Path[strings.LastIndex(u.Path, "/")+1:]
		return browserfetch.NavigatedDocument{
			FinalURL: rawURL,
			HTML: detailPage(detailFixture{
				title: "Arhitect " + id, company: "Firm " + id,
				description: "<p>Row</p>", city: "Cluj", country: "RO", work: "Remote",
			}),
		}, nil
	}
	page := 1
	if i := strings.LastIndex(u.Path, "/pagina"); i >= 0 {
		if _, scanErr := fmt.Sscanf(u.Path[i:], "/pagina%d", &page); scanErr != nil {
			page = 1
		}
	}
	base := strings.TrimRight(strings.TrimSuffix(u.Path, fmt.Sprintf("/pagina%d", page)), "/")
	id := strings.ReplaceAll(strings.Trim(base, "/"), "/", "-")
	return browserfetch.NavigatedDocument{
		FinalURL: rawURL,
		HTML: listHTML(40, []card{{
			url:   siteOrigin + "/user/locuri-de-munca/" + id + "/" + fmt.Sprintf("%d", page),
			title: "Arhitect",
		}}, ""),
	}, nil
}
