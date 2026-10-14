package archgee

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

func TestFetchWithQuery_threeQueriesPageAndTeaser(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if strings.Contains(r.URL.Path, "/api/") || strings.Contains(r.URL.RawQuery, "interior-designer") {
			t.Errorf("forbidden %s", r.URL.RequestURI())
			http.NotFound(w, r)
			return
		}
		switch r.URL.Path {
		case "/jobs":
			writePage(t, w, listForQuery(r.URL.RawQuery))
		case "/jobs/named-remote":
			writePage(t, w, detailPage(detailFixture{
				title: "Project Architect", company: "Pavago",
				city: "Brasil", country: "BR", posted: "2026-09-30",
				description: "<p>Lead the <b>drawings</b> …</p>",
				full:        "This is the full posting that must not be stored.",
				meta:        "Project Architect at Pavago in Brasil. Remote | USD 80,000 per year",
				baseSalary:  "1",
				uuid:        "01a109cb-51ef-72c8-8eab-1030155dda54",
				telecommute: true,
				apply:       "/jobs/named-remote/apply",
			}))
		case "/jobs/named-remote-copy":
			writePage(t, w, detailPage(detailFixture{
				title: "Project Architect", company: "Pavago",
				description: "<p>Same role</p>",
				uuid:        "11111111-1111-1111-1111-111111111111",
			}))
		case "/jobs/confidential-hybrid":
			writePage(t, w, detailPage(detailFixture{
				title: "Architect", company: "Confidential",
				description: "<p>Studio teaser</p>",
				meta:        "Architect at Confidential. Hybrid",
				uuid:        "22222222-2222-2222-2222-222222222222",
				baseSalary:  "99999",
			}))
		case "/jobs/named-office":
			writePage(t, w, detailPage(detailFixture{
				title: "Senior Architect", company: "Calibre Search",
				city: "Leeds", country: "gb", posted: "2026-10-11",
				description: "<p>Leeds studio</p>",
				meta:        "Senior Architect at Calibre Search in Leeds. On-site | GBP 40,000 - 47,500 per year",
				uuid:        "01a13823-6d51-70e4-9ad9-e601f3657df2",
			}))
		case "/jobs/badge-remote":
			writePage(t, w, detailPage(detailFixture{
				title: "Remote Architect", company: "Studio North",
				description: "<p>Anywhere</p>",
				uuid:        "33333333-3333-3333-3333-333333333333",
			}))
		case "/jobs/landscape-role":
			writePage(t, w, detailPage(detailFixture{
				title: "Landscape Architect", company: "Field Office",
				description: "<p>Planting</p>",
				uuid:        "44444444-4444-4444-4444-444444444444",
			}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Archgee{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)}}
	jobs, err := c.FetchWithQuery(context.Background(), "architect")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 4)

	remote := jobs[0]
	require.Equal(t, "Project Architect", remote.Title)
	require.Equal(t, "Pavago", remote.Company)
	require.Equal(t, siteOrigin+"/jobs/named-remote", remote.URL)
	require.Empty(t, remote.ApplyURL)
	require.Equal(t, "Lead the drawings …", remote.Description)
	require.NotContains(t, remote.Description, "full posting")
	require.Equal(t, "USD 80,000 per year", remote.SalaryRaw)
	require.NotEqual(t, "1", remote.SalaryRaw)
	require.Equal(t, schema.LocationRemote, remote.Location.Type)
	require.Equal(t, []string{"BR"}, remote.Location.Countries)
	require.Equal(t, "Brasil", remote.Location.Raw)
	require.Equal(t, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), remote.PostedAt)
	require.Equal(t, domainutils.StableJobID(remote.CompanyKey, remote.Title), remote.ID)
	require.NotContains(t, remote.ID, "01a109cb-51ef-72c8-8eab-1030155dda54")
	require.NotContains(t, remote.ID, remote.URL)

	hybrid := jobs[1]
	require.Equal(t, schema.LocationHybrid, hybrid.Location.Type)
	require.Empty(t, hybrid.SalaryRaw)
	require.Empty(t, hybrid.ApplyURL)
	require.Equal(t, "Studio teaser", hybrid.Description)
	require.Contains(t, hybrid.ID, "https://archgee.com/jobs/confidential-hybrid")
	require.NotEqual(t, "22222222-2222-2222-2222-222222222222", hybrid.ID)

	office := jobs[2]
	require.Equal(t, schema.LocationOffice, office.Location.Type)
	require.Equal(t, "GBP 40,000 - 47,500 per year", office.SalaryRaw)
	require.Equal(t, []string{"GB"}, office.Location.Countries)
	require.Equal(t, "Leeds", office.Location.Raw)
	require.Equal(t, domainutils.StableJobID(office.CompanyKey, office.Title), office.ID)
	require.NotContains(t, office.ID, "01a13823-6d51-70e4-9ad9-e601f3657df2")

	badgeRemote := jobs[3]
	require.Equal(t, schema.LocationRemote, badgeRemote.Location.Type)
	require.Equal(t, "Studio North", badgeRemote.Company)
	require.NotContains(t, badgeRemote.ID, badgeRemote.URL)

	var lists []string
	var details []string
	for _, raw := range urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.NotContains(t, u.Path, "/api/")
		require.NotContains(t, u.RawQuery, "interior-designer")
		require.NotContains(t, u.Path, "/apply")
		switch {
		case u.Path == "/jobs":
			lists = append(lists, u.Scheme+"://"+u.Host+u.RequestURI())
		case strings.HasPrefix(u.Path, "/jobs/"):
			details = append(details, u.Scheme+"://"+u.Host+u.Path)
		default:
			t.Errorf("unexpected url %s", raw)
		}
	}
	require.Equal(t, []string{
		siteOrigin + "/jobs?category=architect",
		siteOrigin + "/jobs?category=architect&page=2",
	}, lists)
	require.Equal(t, []string{
		siteOrigin + "/jobs/named-remote",
		siteOrigin + "/jobs/confidential-hybrid",
		siteOrigin + "/jobs/named-office",
		siteOrigin + "/jobs/badge-remote",
		siteOrigin + "/jobs/named-remote-copy",
	}, details)
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}

	mu.Lock()
	urls = nil
	mu.Unlock()
	landscape, err := c.FetchWithQuery(context.Background(), "landscape-architect")
	require.NoError(t, err)
	require.Len(t, landscape, 1)
	require.Equal(t, "Landscape Architect", landscape[0].Title)
	require.Equal(t, siteOrigin+"/jobs/landscape-role", landscape[0].URL)

	remoteJobs, err := c.FetchWithQuery(context.Background(), "remote")
	require.NoError(t, err)
	require.Empty(t, remoteJobs)

	mu.Lock()
	got := append([]string(nil), urls...)
	mu.Unlock()
	require.Equal(t, []string{
		siteOrigin + "/jobs?category=landscape-architect",
		siteOrigin + "/jobs/landscape-role",
		siteOrigin + "/jobs?remote=remote",
	}, got)
}

func TestFetchWithQuery_rejectsOtherQueries(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected %s", r.URL.RequestURI())
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	c := &Archgee{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}}
	for _, query := range []string{"", "catalog", "interior-designer", "Interior-Designer", "designer de interior", "category=architect"} {
		jobs, err := c.FetchWithQuery(context.Background(), query)
		require.NoError(t, err)
		require.Empty(t, jobs)
	}
	fetched, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, fetched)
	require.Empty(t, urls)
}

func listForQuery(rawQuery string) string {
	switch rawQuery {
	case "category=architect":
		return listPage(true, []card{
			{href: "/jobs/named-remote/?ref=1", badge: "On-site"},
			{href: "/jobs/named-remote/", badge: "On-site"},
			{href: "/jobs/confidential-hybrid", badge: "Hybrid"},
			{href: "/jobs/named-office", badge: "On-site"},
			{href: "https://archgee.com/jobs/badge-remote", badge: "Remote"},
			{href: "/jobs/post", badge: "Remote"},
			{href: "/api/jobs/1", badge: "Remote"},
			{href: "/jobs?category=interior-designer", badge: "Remote"},
		})
	case "category=architect&page=2":
		return listPage(false, []card{
			{href: "/jobs/named-remote", badge: "On-site"},
			{href: "/jobs/named-remote-copy", badge: "On-site"},
		})
	case "category=landscape-architect":
		return listPage(false, []card{{href: "/jobs/landscape-role", badge: "On-site"}})
	case "remote=remote":
		return listPage(false, nil)
	default:
		return `<html><body>unexpected</body></html>`
	}
}

type card struct {
	href  string
	badge string
}

func listPage(next bool, cards []card) string {
	var b strings.Builder
	b.WriteString(`<html><body><div class="space-y-3">`)
	for _, card := range cards {
		b.WriteString(`<a href="`)
		b.WriteString(card.href)
		b.WriteString(`" class="block job-tile"><h2>Role</h2><span>`)
		b.WriteString(card.badge)
		b.WriteString(`</span></a>`)
	}
	b.WriteString(`<a href="/jobs/nav-decoy">nav</a>`)
	b.WriteString(`<a href="https://archgee.com/api/jobs">api</a>`)
	if next {
		b.WriteString(`<a href="https://archgee.com/api/jobs?page=2" rel="next">next api</a>`)
		b.WriteString(`<a href="https://archgee.com/jobs?category=interior-designer&amp;page=2" rel="next">interior</a>`)
	}
	b.WriteString(`</div></body></html>`)
	return b.String()
}

type detailFixture struct {
	title       string
	company     string
	city        string
	country     string
	posted      string
	description string
	full        string
	meta        string
	baseSalary  string
	uuid        string
	telecommute bool
	apply       string
}

func detailPage(f detailFixture) string {
	var b strings.Builder
	b.WriteString(`<html><head>`)
	if f.meta != "" {
		b.WriteString(`<meta name="description" content="`)
		b.WriteString(f.meta)
		b.WriteString(`">`)
	}
	b.WriteString(`</head><body>`)
	if f.full != "" {
		b.WriteString(`<div class="job-description">`)
		b.WriteString(f.full)
		b.WriteString(`</div>`)
	}
	if f.apply != "" {
		b.WriteString(`<form action="`)
		b.WriteString(f.apply)
		b.WriteString(`" method="post"><button>Apply Now</button></form>`)
		b.WriteString(`<a href="https://ats.example/apply">http apply</a>`)
	}
	b.WriteString(`<script type="application/ld+json">{"@type":"BreadcrumbList","name":"Jobs"}</script>`)
	b.WriteString(`<script type="application/ld+json">{"@type":"JobPosting","title":`)
	b.WriteString(jsonString(f.title))
	b.WriteString(`,"description":`)
	b.WriteString(jsonString(f.description))
	b.WriteString(`,"identifier":{"@type":"PropertyValue","name":"ArchGee","value":`)
	b.WriteString(jsonString(f.uuid))
	b.WriteString(`},"hiringOrganization":{"@type":"Organization","name":`)
	b.WriteString(jsonString(f.company))
	b.WriteString(`},"datePosted":`)
	b.WriteString(jsonString(f.posted))
	if f.telecommute {
		b.WriteString(`,"jobLocationType":"TELECOMMUTE"`)
	}
	if f.baseSalary != "" {
		b.WriteString(`,"baseSalary":{"value":{"value":`)
		b.WriteString(jsonString(f.baseSalary))
		b.WriteString(`}}`)
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
