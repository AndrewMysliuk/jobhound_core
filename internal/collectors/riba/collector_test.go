package riba

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
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetchWithQuery_categoryPagesStopWithoutNext(t *testing.T) {
	const slug = "architect"
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
		case "/jobs/" + slug + "/":
			writePage(t, w, listPage([]card{
				{id: "4401", slug: "senior-architect", title: "Senior"},
				{id: "4402", slug: "assistant-architect", title: "Assistant"},
			}, true))
		case "/jobs/" + slug + "/2/":
			writePage(t, w, listPage([]card{
				{id: "4401", slug: "senior-architect", title: "Senior"},
				{id: "4403", slug: "landscape-architect", title: "Landscape"},
			}, false))
		case "/job/4401/senior-architect":
			writePage(t, w, detailPage(detailFixture{
				title: "Senior Architect", company: "Foster + Partners",
				city: "London", country: "GB", posted: "2026-10-02",
				description: "<p>Lead the <b>facade</b></p>",
				salary:      "£45,000 - £55,000",
				salaryLD:    `"9999 GBP"`,
				gtmTitle:    "Wrong Title From GTM",
				gtmCompany:  "Wrong Co",
				gtmSalary:   "£1",
				telecommute: true,
			}))
		case "/job/4402/assistant-architect":
			writePage(t, w, detailPage(detailFixture{
				title: "Architectural Assistant", company: "Confidential",
				description: "<p>Studio work</p>",
				gtmTitle:    "Other GTM Job",
			}))
		case "/job/4403/landscape-architect":
			writePage(t, w, detailPage(detailFixture{
				title: "Landscape Architect", company: "Studio North",
				city: "Manchester", country: "gb",
				description: "<p>Park</p>",
				salary:      "Competitive",
			}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Riba{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)}}
	jobs, err := c.FetchWithQuery(context.Background(), slug)
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 3)

	var lists []string
	for _, raw := range urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.NotContains(t, u.Path, "/romania")
		require.NotContains(t, u.Path, "employment-type")
		require.NotContains(t, u.Path, "recruiter")
		require.NotEqual(t, "/jobs/remote", strings.TrimSuffix(u.Path, "/"))
		require.NotEqual(t, "/jobs/remote/", u.Path)
		require.NotContains(t, u.RawQuery, "TrackID")
		require.NotContains(t, strings.ToLower(u.RawQuery), "utm_")
		require.NotContains(t, raw, "/jobs/"+slug+"/3/")
		if strings.Contains(u.Path, "/jobs/") {
			lists = append(lists, u.Scheme+"://"+u.Host+u.Path)
		}
	}
	base := siteOrigin + "/jobs/" + slug + "/"
	require.Equal(t, []string{base, base + "2/"}, lists)
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}

	named := jobs[0]
	require.Equal(t, "Senior Architect", named.Title)
	require.NotEqual(t, "Wrong Title From GTM", named.Title)
	require.Equal(t, "Foster + Partners", named.Company)
	require.NotEqual(t, "Wrong Co", named.Company)
	require.Equal(t, "https://jobs.architecture.com/job/4401/senior-architect", named.URL)
	require.Equal(t, named.URL, named.ApplyURL)
	require.NotContains(t, named.URL, "TrackID")
	require.NotContains(t, named.URL, "utm_")
	require.Equal(t, "Lead the facade", named.Description)
	require.Equal(t, "£45,000 - £55,000", named.SalaryRaw)
	require.NotContains(t, named.SalaryRaw, "9999")
	require.NotEqual(t, "£1", named.SalaryRaw)
	require.Empty(t, named.Location.Type)
	require.Equal(t, []string{"GB"}, named.Location.Countries)
	require.Equal(t, "London", named.Location.Raw)
	require.Equal(t, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), named.PostedAt)
	require.Equal(t, domainutils.StableJobID(named.CompanyKey, named.Title), named.ID)
	require.NotContains(t, named.ID, named.URL)

	placeholder := jobs[1]
	require.Equal(t, "Architectural Assistant", placeholder.Title)
	require.Empty(t, placeholder.SalaryRaw)
	require.Empty(t, placeholder.Location.Type)
	require.Equal(t, placeholder.URL, placeholder.ApplyURL)
	require.Contains(t, placeholder.ID, placeholder.URL)

	office := jobs[2]
	require.Equal(t, "Studio North", office.Company)
	require.Equal(t, "Competitive", office.SalaryRaw)
	require.Empty(t, office.Location.Type)
	require.Equal(t, []string{"GB"}, office.Location.Countries)
	require.NotContains(t, office.ID, office.URL)
}

func TestFetchWithQuery_noNextSkipsSecondPage(t *testing.T) {
	const slug = "part-1"
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/jobs/" + slug + "/":
			writePage(t, w, listPage([]card{{id: "12", slug: "part-one", title: "Part"}}, false))
		case "/job/12/part-one":
			writePage(t, w, detailPage(detailFixture{title: "Part I", company: "Firm"}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Riba{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}}
	jobs, err := c.FetchWithQuery(context.Background(), slug)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	for _, raw := range urls {
		require.NotContains(t, raw, "/2/")
	}
}

func TestFetch_rejectsUnfilteredAndFacetQueries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	c := &Riba{HTTPClient: srv.Client()}
	for _, query := range []string{"", "remote", "romania", "architect/romania", "employment-type", "architect/salary/40000", "recruiter"} {
		jobs, err := c.FetchWithQuery(context.Background(), query)
		require.NoError(t, err)
		require.Empty(t, jobs)
	}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, jobs)
}

type card struct {
	id    string
	slug  string
	title string
}

func listPage(cards []card, next bool) string {
	var b strings.Builder
	b.WriteString(`<html><head>`)
	if next {
		b.WriteString(`<link rel="next" href="https://jobs.architecture.com/jobs/architect/romania/">`)
		b.WriteString(`<link rel="next" href="https://jobs.architecture.com/jobs/architect/employment-type/permanent/">`)
	}
	b.WriteString(`</head><body>`)
	b.WriteString(`<a href="/jobs/remote/">Remote</a>`)
	b.WriteString(`<a href="/jobs/architect/recruiter/acme/">Recruiter</a>`)
	b.WriteString(`<a href="/jobs/architect/salary/40000/">Salary facet</a>`)
	for _, card := range cards {
		b.WriteString(`<a href="/job/`)
		b.WriteString(card.id)
		b.WriteString(`/`)
		b.WriteString(card.slug)
		b.WriteString(`/?TrackID=99&utm_source=board&utm_medium=list">`)
		b.WriteString(card.title)
		b.WriteString(`</a>`)
	}
	b.WriteString(`</body></html>`)
	return b.String()
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
	gtmTitle    string
	gtmCompany  string
	gtmSalary   string
	telecommute bool
}

func detailPage(f detailFixture) string {
	var b strings.Builder
	b.WriteString(`<html><body>`)
	b.WriteString(`<script>window.dataLayer=window.dataLayer||[];dataLayer.push({`)
	b.WriteString(`"jobId":"999","jobTitle":`)
	b.WriteString(jsonString(f.gtmTitle))
	b.WriteString(`,"company":`)
	b.WriteString(jsonString(f.gtmCompany))
	b.WriteString(`,"salary":`)
	b.WriteString(jsonString(f.gtmSalary))
	b.WriteString(`,"city":"Wrong City"});</script>`)
	b.WriteString(`<dl><dt>Location</dt><dd>Remote, UK</dd><dt>Salary:</dt><dd>`)
	b.WriteString(f.salary)
	b.WriteString(`</dd></dl>`)
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
