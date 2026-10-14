package archinect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetchWithQuery_remotePagesAndVisibleFields(t *testing.T) {
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
			t.Errorf("query %s", r.URL.RawQuery)
			http.Error(w, "query", http.StatusBadRequest)
			return
		}
		switch r.URL.Path {
		case remotePath:
			writePage(t, w, listPage(37, []card{
				{href: "/jobs/entry/150568289/architectural-designer?utm_source=board", title: "Architectural Designer", company: "Clawson Architects", location: "Maplewood, NJ, US (remote possible)", posted: "Thu, Oct 1 '26", pay: "$70,000 - $100,000 annually"},
				{href: "/jobs/entry/150568289/architectural-designer", title: "Architectural Designer", company: "Clawson Architects", location: "Maplewood, NJ, US (remote possible)", posted: "Thu, Oct 1 '26", pay: "$70,000 - $100,000 annually"},
				{href: "/jobs/entry/150568867/job-captain", title: "Job Captain", company: "Confidential", location: "Los Angeles, CA, US (remote possible)", posted: "Fri, Oct 9 '26", pay: "Full-time"},
				{href: "https://www.archinect.com/jobs/entry/150569125/representation-specialist", title: "Representation Specialist", company: "Archive Owl", location: "Remote", posted: "Thu, Oct 8 '26", pay: "$25 - $40 hourly"},
			}, true))
		case remotePath + "/30":
			writePage(t, w, listPage(37, []card{
				{href: "/jobs/entry/150564089/design-staff", title: "Architectural Design Staff", company: "blinkLAB architecture", location: "Oakland, CA, US (remote possible)", posted: "Mon, Sep 7 '26", pay: "$120,000 - $155,000 annually"},
			}, false))
		case "/jobs/entry/150568289/architectural-designer":
			writePage(t, w, detailPage(detailFixture{
				title: "Architectural Designer", company: "Clawson Architects",
				location: "Maplewood, NJ, US (remote possible)", posted: "Thu, Oct 1 '26",
				pay:         "$70,000 - $100,000 annually",
				description: "<p>Position Overview</p>\n<p>Seeking a <b>designer</b> for the studio.</p>\n<p>Write to <span class=\"__cf_email__\" data-cfemail=\"0d7f6863684d6e616c7a7e62636c7f6e656479686e797e236e6260\">[email&#160;protected]</span>.</p>",
			}))
		case "/jobs/entry/150568867/job-captain":
			writePage(t, w, detailPage(detailFixture{
				title: "Job Captain", company: "Confidential",
				location:    "Los Angeles, CA, US (remote possible)",
				description: "<p>Contract role in the office two days a week.</p>",
			}))
		case "/jobs/entry/150569125/representation-specialist":
			writePage(t, w, detailPage(detailFixture{
				title: "Representation Specialist", company: "Archive Owl",
				description: "<p>Model the work.</p>",
			}))
		case "/jobs/entry/150564089/design-staff":
			writePage(t, w, detailPage(detailFixture{
				title: "Architectural Design Staff", company: "blinkLAB architecture",
				location: "Oakland, CA, US (remote possible)", posted: "Mon, Sep 7 '26",
				pay:         "$120,000 - $155,000 annually",
				description: "<p>Lead a small team.</p>",
			}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Archinect{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)}, Gap: -1}
	jobs, err := c.FetchWithQuery(context.Background(), "remote")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 4)

	named := jobs[0]
	require.Equal(t, "Architectural Designer", named.Title)
	require.Equal(t, "Clawson Architects", named.Company)
	require.Equal(t, siteOrigin+"/jobs/entry/150568289/architectural-designer", named.URL)
	require.NotContains(t, named.URL, "utm_source")
	require.Empty(t, named.ApplyURL)
	require.Equal(t, "Position Overview Seeking a designer for the studio. Write to [email protected].", named.Description)
	require.NotContains(t, named.Description, "rene@clawsonarchitects.com")
	require.NotContains(t, named.Description, "JSON-LD decoy")
	require.NotContains(t, named.Description, "Bump Up")
	require.Equal(t, "$70,000 - $100,000 annually", named.SalaryRaw)
	require.NotEqual(t, "1", named.SalaryRaw)
	require.Equal(t, schema.LocationRemote, named.Location.Type)
	require.Empty(t, named.Location.Countries)
	require.Equal(t, "Maplewood, NJ, US (remote possible)", named.Location.Raw)
	require.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), named.PostedAt)
	require.NotEqual(t, time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC), named.PostedAt)
	require.Equal(t, domainutils.StableJobID(named.CompanyKey, named.Title), named.ID)
	require.NotContains(t, named.ID, named.URL)
	require.NotEqual(t, "150568289", named.ID)

	confidential := jobs[1]
	require.Equal(t, "Confidential", confidential.Company)
	require.Empty(t, confidential.SalaryRaw)
	require.Empty(t, confidential.ApplyURL)
	require.Equal(t, schema.LocationRemote, confidential.Location.Type)
	require.Equal(t, "Contract role in the office two days a week.", confidential.Description)
	require.Contains(t, confidential.ID, confidential.URL)
	require.Equal(t, time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), confidential.PostedAt)

	cardPay := jobs[2]
	require.Equal(t, "$25 - $40 hourly", cardPay.SalaryRaw)
	require.Equal(t, "Remote", cardPay.Location.Raw)
	require.Equal(t, schema.LocationRemote, cardPay.Location.Type)
	require.Equal(t, time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), cardPay.PostedAt)
	require.NotContains(t, cardPay.ID, cardPay.URL)

	pageTwo := jobs[3]
	require.Equal(t, "Architectural Design Staff", pageTwo.Title)
	require.Equal(t, siteOrigin+"/jobs/entry/150564089/design-staff", pageTwo.URL)
	require.Equal(t, schema.LocationRemote, pageTwo.Location.Type)

	var lists []string
	var details []string
	for _, raw := range urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.Empty(t, u.RawQuery)
		require.NotContains(t, u.Path, "/jobs/region/DK")
		require.NotContains(t, u.Path, "/jobs/region/US")
		require.NotEqual(t, "/jobs", strings.TrimRight(u.Path, "/"))
		switch {
		case u.Path == remotePath || strings.HasPrefix(u.Path, remotePath+"/"):
			lists = append(lists, u.Scheme+"://"+u.Host+u.Path)
		case strings.HasPrefix(u.Path, "/jobs/entry/"):
			details = append(details, u.Scheme+"://"+u.Host+u.Path)
		default:
			t.Errorf("unexpected url %s", raw)
		}
	}
	require.Equal(t, []string{
		siteOrigin + remotePath,
		siteOrigin + remotePath + "/30",
	}, lists)
	require.Equal(t, []string{
		siteOrigin + "/jobs/entry/150568289/architectural-designer",
		siteOrigin + "/jobs/entry/150568867/job-captain",
		siteOrigin + "/jobs/entry/150569125/representation-specialist",
		siteOrigin + "/jobs/entry/150564089/design-staff",
	}, details)
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}
}

func TestFetchWithQuery_stopsWhenOffsetReachesTotal(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case remotePath:
			writePage(t, w, listPage(10, []card{
				{href: "/jobs/entry/1/one", title: "One", company: "Firm", pay: "Full-time"},
			}, true))
		case "/jobs/entry/1/one":
			writePage(t, w, detailPage(detailFixture{title: "One", company: "Firm", description: "<p>Body</p>"}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Archinect{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}, Gap: -1}
	jobs, err := c.FetchWithQuery(context.Background(), "Remote")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, []string{
		siteOrigin + remotePath,
		siteOrigin + "/jobs/entry/1/one",
	}, urls)
}

func TestFetchWithQuery_stopsOnEmptyEntries(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case remotePath:
			writePage(t, w, listPage(90, []card{
				{href: "/jobs/entry/2/two", title: "Two", company: "Firm"},
			}, false))
		case remotePath + "/30":
			writePage(t, w, listPage(90, nil, false))
		case "/jobs/entry/2/two":
			writePage(t, w, detailPage(detailFixture{title: "Two", company: "Firm", description: "<p>Body</p>"}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Archinect{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}, Gap: -1}
	jobs, err := c.FetchWithQuery(context.Background(), "remote")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, []string{
		siteOrigin + remotePath,
		siteOrigin + "/jobs/entry/2/two",
		siteOrigin + remotePath + "/30",
	}, urls)
}

func TestFetchWithQuery_rejectsOtherQueries(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected %s", r.URL.RequestURI())
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	c := &Archinect{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}, Gap: -1}
	for _, query := range []string{"", "catalog", "DK", "US", "united-states", "denmark", "/jobs", "remote/30"} {
		jobs, err := c.FetchWithQuery(context.Background(), query)
		require.NoError(t, err)
		require.Empty(t, jobs)
	}
	fetched, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, fetched)
	require.Empty(t, urls)
}

func TestGap_atLeastFiveSeconds(t *testing.T) {
	require.GreaterOrEqual(t, (&Archinect{}).gap(), 5*time.Second)
	require.Equal(t, time.Duration(0), (&Archinect{Gap: -1}).gap())
}

type card struct {
	href     string
	title    string
	company  string
	location string
	posted   string
	pay      string
}

func listPage(total int, cards []card, decoys bool) string {
	var b strings.Builder
	b.WriteString(`<html><body><ul class="JobList">`)
	for _, card := range cards {
		b.WriteString(`<li class="Entry "><a href="`)
		b.WriteString(card.href)
		b.WriteString(`"><div class="Col2"><p>`)
		b.WriteString(card.location)
		b.WriteString(`</p><span class="">`)
		b.WriteString(card.posted)
		b.WriteString(`</span></div><div class="Col1"><h1>`)
		b.WriteString(card.title)
		b.WriteString(`</h1><h2>`)
		b.WriteString(card.company)
		b.WriteString(`</h2><h3>`)
		b.WriteString(card.pay)
		b.WriteString(`</h3></div></a></li>`)
	}
	b.WriteString(`</ul>`)
	if decoys {
		b.WriteString(`<a href="/jobs">Worldwide</a>`)
		b.WriteString(`<a href="/jobs/region/DK/all/denmark">Denmark</a>`)
		b.WriteString(`<a href="/jobs/region/US/all/united-states">United States</a>`)
		b.WriteString(`<a href="/jobs/region/Remote/all/remote/60">3</a>`)
		b.WriteString(`<a href="/jobs/entry/999/outside-the-list">outside</a>`)
	}
	b.WriteString(`<span style="display:none" id="arc_tr">`)
	b.WriteString(strconv.Itoa(total))
	b.WriteString(`</span></body></html>`)
	return b.String()
}

type detailFixture struct {
	title       string
	company     string
	location    string
	posted      string
	pay         string
	description string
}

func detailPage(f detailFixture) string {
	var b strings.Builder
	b.WriteString(`<html><body><div class="JobDescription NewStyle"><div class="Head">`)
	b.WriteString(`<span class="arc_dynamic_edit"><!-- Bump Up --></span><h1>`)
	b.WriteString(f.title)
	b.WriteString(`</h1><div class="JobInfo ">`)
	if f.company != "" {
		b.WriteString(`<span class="Right"><a href="/firms/cover/1/firm">`)
		b.WriteString(f.company)
		b.WriteString(`</a></span><span class="Left">Employer:</span>`)
	}
	if f.location != "" {
		b.WriteString(`<span class="Right">`)
		b.WriteString(f.location)
		b.WriteString(`</span><span class="Left">Location:</span>`)
	}
	if f.posted != "" {
		b.WriteString(`<span class="Right">`)
		b.WriteString(f.posted)
		b.WriteString(`</span><span class="Left">Posted on:</span>`)
	}
	if f.pay != "" {
		b.WriteString(`<span class="Right">`)
		b.WriteString(f.pay)
		b.WriteString(`</span><span class="Left">Pay:</span>`)
	}
	b.WriteString(`</div></div>`)
	b.WriteString(f.description)
	b.WriteString(`<script type="application/ld+json">{
  "@type": "JobPosting",
  "title": "JSON Title",
  "description": "JSON-LD decoy
that must not be stored",
  "datePosted": "1999-01-01",
  "baseSalary": "1",
  "email": "mailto:decoy@json.example"
}</script>`)
	b.WriteString(`</div><p class="back-to-list"><a href="/jobs">Back to Job List...</a></p></body></html>`)
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
