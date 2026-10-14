package landscapeinstitute

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

func TestFetch_singleCatalogAndListCardWorkplace(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.RawQuery != "" {
			t.Errorf("query %s on %s", r.URL.RawQuery, r.URL.Path)
			http.Error(w, "query", http.StatusBadRequest)
			return
		}
		switch r.URL.Path {
		case "/jobs/":
			writePage(t, w, listPage())
		case "/job/remote-role":
			writePage(t, w, detailPage(detailFixture{
				title: "Landscape Architect", company: "PAD landscapes",
				city: "London", country: "GB", posted: "2026-10-04T11:33:45+01:00",
				description: "&lt;p&gt;Lead the &lt;b&gt;park&lt;/b&gt;&lt;/p&gt;",
				salary:      "£45,000 - £55,000",
				mailto:      "mailto:en&#113;uiries&#64;firm.example?subject=Application%20via%20role",
				mailtoDecoy: "mailto:other&#64;decoy.example",
				regionClass: "job_listing_region-greaterlondon",
			}))
		case "/job/hybrid-role":
			writePage(t, w, detailPage(detailFixture{
				title: "Architectural Assistant", company: "Confidential",
				description: "<p>Studio</p>",
				salaryFlat:  "9999",
			}))
		case "/job/home-role":
			writePage(t, w, detailPage(detailFixture{
				title: "Home Based Designer", company: "Studio North",
				description: "<p>Planting</p>",
				salaryText:  "45000",
			}))
		case "/job/london-role":
			writePage(t, w, detailPage(detailFixture{
				title: "Senior Landscape Architect", company: "Jinny Blom Ltd",
				addressText: "South East",
				description: "<p>Clerkenwell</p>",
				regionClass: "job_listing_region-remotely",
				telecommute: true,
			}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &LandscapeInstitute{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)}}
	jobs, err := c.FetchWithQuery(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 4)

	var lists []string
	var details []string
	for _, raw := range urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.Empty(t, u.RawQuery)
		require.NotContains(t, u.Path, "/page/")
		require.NotContains(t, raw, "search_keywords")
		require.NotContains(t, raw, "search_region")
		require.NotContains(t, u.Path, "nav-decoy")
		switch {
		case u.Path == "/jobs/" || strings.HasPrefix(u.Path, "/jobs/"):
			lists = append(lists, u.Scheme+"://"+u.Host+u.Path)
		case strings.HasPrefix(u.Path, "/job/"):
			details = append(details, u.Scheme+"://"+u.Host+u.Path)
		default:
			t.Errorf("unexpected url %s", raw)
		}
	}
	require.Equal(t, []string{listURL}, lists)
	require.Equal(t, []string{
		siteOrigin + "/job/remote-role",
		siteOrigin + "/job/hybrid-role",
		siteOrigin + "/job/home-role",
		siteOrigin + "/job/london-role",
	}, details)
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}

	remote := jobs[0]
	require.Equal(t, "Landscape Architect", remote.Title)
	require.Equal(t, "PAD landscapes", remote.Company)
	require.Equal(t, siteOrigin+"/job/remote-role", remote.URL)
	require.NotContains(t, remote.URL, "search_region")
	require.NotContains(t, remote.URL, "search_keywords")
	require.Equal(t, "mailto:enquiries@firm.example?subject=Application%20via%20role", remote.ApplyURL)
	require.NotContains(t, remote.ApplyURL, "decoy")
	require.Equal(t, "Lead the park", remote.Description)
	require.Equal(t, "£45,000 - £55,000", remote.SalaryRaw)
	require.Equal(t, schema.LocationRemote, remote.Location.Type)
	require.Equal(t, []string{"GB"}, remote.Location.Countries)
	require.Equal(t, "London", remote.Location.Raw)
	require.Equal(t, time.Date(2026, 10, 4, 10, 33, 45, 0, time.UTC), remote.PostedAt)
	require.Equal(t, domainutils.StableJobID(remote.CompanyKey, remote.Title), remote.ID)
	require.NotContains(t, remote.ID, remote.URL)
	require.NotEqual(t, "12131", remote.ID)

	hybrid := jobs[1]
	require.Equal(t, schema.LocationHybrid, hybrid.Location.Type)
	require.Empty(t, hybrid.SalaryRaw)
	require.Empty(t, hybrid.ApplyURL)
	require.Empty(t, hybrid.PostedAt)
	require.Contains(t, hybrid.ID, hybrid.URL)

	home := jobs[2]
	require.Equal(t, schema.LocationHybrid, home.Location.Type)
	require.Empty(t, home.SalaryRaw)
	require.Equal(t, "Studio North", home.Company)
	require.NotContains(t, home.ID, home.URL)

	office := jobs[3]
	require.Empty(t, office.Location.Type)
	require.Empty(t, office.Location.Countries)
	require.Equal(t, "South East", office.Location.Raw)
	require.Equal(t, "Jinny Blom Ltd", office.Company)
	require.NotContains(t, office.ID, office.URL)
}

func TestFetchWithQuery_ignoresQueryAndSecondPage(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs/" || r.URL.RawQuery != "" {
			t.Errorf("unexpected %s", r.URL.RequestURI())
			http.NotFound(w, r)
			return
		}
		writePage(t, w, `<html><body><ul class="job_listings"></ul><a href="/jobs/page/2/">2</a></body></html>`)
	}))
	t.Cleanup(srv.Close)

	c := &LandscapeInstitute{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}}
	for _, query := range []string{"", "catalog", "peisagist", "search_region=1153"} {
		jobs, err := c.FetchWithQuery(context.Background(), query)
		require.NoError(t, err)
		require.Empty(t, jobs)
	}
	fetched, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, fetched)
	require.Len(t, urls, 5)
	for _, raw := range urls {
		require.Equal(t, listURL, raw)
	}
}

func listPage() string {
	return `<html><body>
<ul class="job_listings">
<li id="job_listing-12131" class="job_listing job_listing_region-remotely job_listing_region-greaterlondon">
<a href="/job/remote-role/?search_region=1153&amp;search_keywords=arhitect">Remote</a>
</li>
<li class="job_listing job_listing_region-remotely">
<a href="/job/remote-role/">Remote again</a>
</li>
<li class="job_listing job_listing_region-hybrid">
<a href="https://jobs.landscapeinstitute.org/job/hybrid-role/">Hybrid</a>
</li>
<li class="job_listing job_listing_region-homebased">
<a href="/job/home-role/">Home</a>
</li>
<li class="job_listing job_listing_region-greaterlondon job_listing_type-full-time">
<a href="/job/london-role/">London</a>
</li>
</ul>
<div class="job_listing-title"><a href="/job/nav-decoy/">nav</a></div>
<a href="/jobs/page/2/">page 2</a>
<a href="/jobs/?search_keywords=arhitect&amp;search_region=1153">search</a>
<a href="https://jobs.landscapeinstitute.org/feed/">feed</a>
</body></html>`
}

type detailFixture struct {
	title       string
	company     string
	city        string
	country     string
	addressText string
	posted      string
	description string
	salary      string
	salaryFlat  string
	salaryText  string
	mailto      string
	mailtoDecoy string
	regionClass string
	telecommute bool
}

func detailPage(f detailFixture) string {
	var b strings.Builder
	b.WriteString(`<html><body>`)
	if f.regionClass != "" {
		b.WriteString(`<li class="job_listing `)
		b.WriteString(f.regionClass)
		b.WriteString(`"></li>`)
	}
	b.WriteString(`<script type="application/ld+json" class="aioseo-schema">{"@context":"https://schema.org","@graph":[{"@type":"BreadcrumbList","name":"Jobs"}]}</script>`)
	if f.mailto != "" {
		b.WriteString(`<a class="job_application_email" href="`)
		b.WriteString(f.mailto)
		b.WriteString(`">apply</a>`)
	}
	if f.mailtoDecoy != "" {
		b.WriteString(`<a href="`)
		b.WriteString(f.mailtoDecoy)
		b.WriteString(`">other</a>`)
	}
	b.WriteString(`<a href="https://ats.example/apply">http apply</a>`)
	b.WriteString(`<script type="application/ld+json">`)
	b.WriteString(`{"@type":"JobPosting","title":`)
	b.WriteString(jsonString(f.title))
	b.WriteString(`,"description":`)
	b.WriteString(jsonString(f.description))
	b.WriteString(`,"hiringOrganization":{"@type":"Organization","name":`)
	b.WriteString(jsonString(f.company))
	b.WriteString(`},"datePosted":`)
	b.WriteString(jsonString(f.posted))
	b.WriteString(`,"identifier":"12131","baseSalary":`)
	switch {
	case f.salary != "":
		b.WriteString(`{"@type":"MonetaryAmount","currency":"GBP","value":{"@type":"QuantitativeValue","value":`)
		b.WriteString(jsonString(f.salary))
		b.WriteString(`,"unitText":"YEAR"}}`)
	case f.salaryFlat != "":
		b.WriteString(`{"value":`)
		b.WriteString(jsonString(f.salaryFlat))
		b.WriteString(`}`)
	case f.salaryText != "":
		b.WriteString(jsonString(f.salaryText))
	default:
		b.WriteString(`null`)
	}
	if f.telecommute {
		b.WriteString(`,"jobLocationType":"TELECOMMUTE"`)
	}
	switch {
	case f.addressText != "":
		b.WriteString(`,"jobLocation":{"@type":"Place","address":`)
		b.WriteString(jsonString(f.addressText))
		b.WriteString(`}`)
	case f.city != "" || f.country != "":
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
