package dezeen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetchWithQuery_stopsOnEmptyListAndReadsSalaryFromHTML(t *testing.T) {
	const slug = "architectural-designer"
	var mu sync.Mutex
	var urls []string
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/job-category/" + slug + "/":
			writePage(t, w, listPage([]card{
				{id: "48291", slug: "senior-architectural-designer", title: "Short", location: "london", locationText: "London"},
				{id: "48292", slug: "assistant", title: "Assistant", location: "remote", locationText: "Remote"},
			}, slug+"/feed/"))
		case "/job-category/" + slug + "/page/2/":
			writePage(t, w, listPage([]card{
				{id: "48291", slug: "senior-architectural-designer", title: "Short", location: "london", locationText: "London"},
				{id: "1002", slug: "landscape-architect-paris", title: "Paris", location: "paris", locationText: "Paris"},
			}, slug+"/page/3/"))
		case "/job-category/" + slug + "/page/3/":
			writePage(t, w, emptyPage(slug+"/page/4/"))
		case "/job/senior-architectural-designer-48291":
			writePage(t, w, detailPage(detailFixture{
				title: "Senior Architectural Designer", company: "Foster + Partners",
				city: "London", country: "GB", posted: "2026-10-01",
				description: "<p>Design the <b>facade</b></p>",
				salary:      "£45,000 - £55,000",
				salaryLD:    `{"currency":"USD","value":"choose_my_own"}`,
				telecommute: true,
				apply:       "https://ats.example/apply/1",
			}))
		case "/job/assistant-48292":
			writePage(t, w, detailPage(detailFixture{
				title: "Architectural Assistant", company: "Confidential",
				description: "<p>Studio work</p>",
				salaryLD:    `"9999 USD"`,
				apply:       "https://www.dezeenjobs.com/login/",
				applyNote:   "Please sign-in to apply.",
			}))
		case "/job/landscape-architect-paris-1002":
			writePage(t, w, detailPage(detailFixture{
				title: "Landscape Architect", company: "Studio North",
				city: "Paris", country: "FR",
				description: "<p>Park</p>",
				salaryLD:    `"9999 USD"`,
				apply:       "https://ats.example/apply/9",
				applyNote:   "A geo gate hides this vacancy.",
			}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Dezeen{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)}}
	jobs, err := c.FetchWithQuery(context.Background(), slug)
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 3)

	var lists []string
	for _, raw := range urls {
		require.NotContains(t, raw, "/feed/")
		require.NotContains(t, raw, "algolia-hit")
		require.NotContains(t, raw, "nav-job")
		require.NotContains(t, raw, "/page/4/")
		if strings.Contains(raw, "/job-category/") {
			lists = append(lists, raw)
		}
	}
	base := siteOrigin + "/job-category/" + slug + "/"
	require.Equal(t, []string{
		base,
		base + "page/2/",
		base + "page/3/",
	}, lists)
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}

	named := jobs[0]
	require.Equal(t, "Senior Architectural Designer", named.Title)
	require.Equal(t, "Foster + Partners", named.Company)
	require.Equal(t, "https://www.dezeenjobs.com/job/senior-architectural-designer-48291", named.URL)
	require.Equal(t, "Design the facade", named.Description)
	require.Equal(t, "£45,000 - £55,000", named.SalaryRaw)
	require.NotContains(t, named.SalaryRaw, "USD")
	require.NotContains(t, named.SalaryRaw, "choose_my_own")
	require.Equal(t, "https://ats.example/apply/1", named.ApplyURL)
	require.Empty(t, named.Location.Type)
	require.Equal(t, []string{"GB"}, named.Location.Countries)
	require.Equal(t, "London", named.Location.Raw)
	require.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), named.PostedAt)
	require.Equal(t, domainutils.StableJobID(named.CompanyKey, named.Title), named.ID)
	require.NotContains(t, named.ID, named.URL)

	remote := jobs[1]
	require.Equal(t, schema.LocationRemote, remote.Location.Type)
	require.Empty(t, remote.SalaryRaw)
	require.NotContains(t, remote.SalaryRaw, "9999")
	require.Empty(t, remote.ApplyURL)
	require.Contains(t, remote.ID, remote.URL)

	gated := jobs[2]
	require.Equal(t, "Studio North", gated.Company)
	require.Empty(t, gated.Location.Type)
	require.Equal(t, []string{"FR"}, gated.Location.Countries)
	require.Empty(t, gated.ApplyURL)
	require.Empty(t, gated.SalaryRaw)
	require.NotContains(t, gated.ID, gated.URL)
}

func TestFetch_emptyQueryDoesNotRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	c := &Dezeen{HTTPClient: srv.Client()}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, jobs)

	jobs, err = c.FetchWithQuery(context.Background(), "feed")
	require.NoError(t, err)
	require.Empty(t, jobs)
}

type card struct {
	id           string
	slug         string
	title        string
	location     string
	locationText string
}

func listPage(cards []card, next string) string {
	var b strings.Builder
	b.WriteString(`<html><head>`)
	b.WriteString(`<link rel="next" href="https://www.dezeenjobs.com/job-category/`)
	b.WriteString(next)
	b.WriteString(`">`)
	b.WriteString(`<link rel="alternate" type="application/rss+xml" href="https://www.dezeenjobs.com/job-category/architectural-designer/feed/">`)
	b.WriteString(`</head><body>`)
	b.WriteString(`<nav><ul class="job_listings"><li class="job_listing"><a href="/job/nav-job-888/">Nav</a></li></ul></nav>`)
	b.WriteString(`<div id="algolia-hits"><ul class="job_listings"><li class="job_listing"><a href="/job/algolia-hit-777/">Algolia</a></li></ul></div>`)
	b.WriteString(`<div class="main-jobs-list"><ul class="job_listings">`)
	for _, card := range cards {
		b.WriteString(`<li class="job_listing"><a href="/job/`)
		b.WriteString(card.slug)
		b.WriteString(`-`)
		b.WriteString(card.id)
		b.WriteString(`/">`)
		b.WriteString(card.title)
		b.WriteString(`</a><div class="location"><a href="/location/`)
		b.WriteString(card.location)
		b.WriteString(`/">`)
		b.WriteString(card.locationText)
		b.WriteString(`</a></div></li>`)
	}
	b.WriteString(`</ul></div></body></html>`)
	return b.String()
}

func emptyPage(next string) string {
	return `<html><head><link rel="next" href="https://www.dezeenjobs.com/job-category/` + next + `"></head><body>` +
		`<div class="main-jobs-list"></div>` +
		`<nav><ul class="job_listings"><li class="job_listing"><a href="/job/should-not-fetch-999/">x</a></li></ul></nav>` +
		`</body></html>`
}

type detailFixture struct {
	title       string
	company     string
	city        string
	country     string
	posted      string
	description string
	salary      string
	salaryLD    string
	telecommute bool
	apply       string
	applyNote   string
}

func detailPage(f detailFixture) string {
	var b strings.Builder
	b.WriteString(`<html><body><div class="single_job_listing">`)
	if f.salary != "" {
		b.WriteString(`<div class="salary-range">`)
		b.WriteString(f.salary)
		b.WriteString(`</div>`)
	}
	b.WriteString(`<div class="job_application application"><p>`)
	b.WriteString(f.applyNote)
	b.WriteString(`</p>`)
	if f.apply != "" {
		b.WriteString(`<a href="`)
		b.WriteString(f.apply)
		b.WriteString(`">Apply</a>`)
	}
	b.WriteString(`<a href="mailto:jobs@firm.example">email</a></div></div>`)
	b.WriteString(`<script type="application/ld+json">`)
	b.WriteString(`{"@type":"JobPosting","title":`)
	b.WriteString(jsonString(f.title))
	b.WriteString(`,"description":`)
	b.WriteString(jsonString(f.description))
	b.WriteString(`,"hiringOrganization":{"@type":"Organization","name":`)
	b.WriteString(jsonString(f.company))
	b.WriteString(`},"datePosted":`)
	b.WriteString(jsonString(f.posted))
	b.WriteString(`,"baseSalary":`)
	if f.salaryLD == "" {
		b.WriteString(`""`)
	} else {
		b.WriteString(f.salaryLD)
	}
	if f.telecommute {
		b.WriteString(`,"jobLocationType":"TELECOMMUTE"`)
	}
	if f.city != "" || f.country != "" {
		b.WriteString(`,"jobLocation":{"@type":"Place","address":{"addressLocality":`)
		b.WriteString(jsonString(f.city))
		b.WriteString(`,"addressCountry":`)
		b.WriteString(jsonString(f.country))
		b.WriteString(`}}`)
	}
	b.WriteString(`}</script></body></html>`)
	return b.String()
}

func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func writePage(t *testing.T, w http.ResponseWriter, html string) {
	t.Helper()
	_, err := w.Write([]byte(html))
	require.NoError(t, err)
}

type recordTransport struct {
	baseURL string
	next    http.RoundTripper
	mu      *sync.Mutex
	urls    *[]string
	agents  *[]string
}

func recordHost(base string, mu *sync.Mutex, urls *[]string, agents *[]string) recordTransport {
	return recordTransport{baseURL: base, next: http.DefaultTransport, mu: mu, urls: urls, agents: agents}
}

func (r recordTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	*r.urls = append(*r.urls, req.URL.String())
	if r.agents != nil {
		*r.agents = append(*r.agents, req.Header.Get("User-Agent"))
	}
	r.mu.Unlock()
	target, err := url.Parse(r.baseURL)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.URL.Scheme = target.Scheme
	clone.URL.Host = target.Host
	clone.Host = target.Host
	return r.next.RoundTrip(clone)
}
