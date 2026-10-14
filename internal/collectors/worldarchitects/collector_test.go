package worldarchitects

import (
	"context"
	"net/http"
	"net/http/cookiejar"
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

func TestFetch_catalogStopsOnRepeatedSlugs(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	var agents []string
	var cookies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if queryHasFold(r.URL, "term") {
			t.Errorf("term %s", r.URL.RawQuery)
			http.Error(w, "term", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "term"})
		switch r.URL.Path {
		case listPath:
			switch r.URL.Query().Get("page") {
			case "":
				_, _ = w.Write([]byte(listPage1))
			case "2":
				_, _ = w.Write([]byte(listPage2))
			case "3":
				_, _ = w.Write([]byte(listPage3))
			default:
				t.Errorf("page %s", r.URL.RawQuery)
				http.NotFound(w, r)
			}
		case listPath + "/alpha":
			_, _ = w.Write([]byte(detailNamed))
		case listPath + "/beta":
			_, _ = w.Write([]byte(detailConfidential))
		case listPath + "/gamma":
			_, _ = w.Write([]byte(detailGermany))
		default:
			t.Errorf("unexpected %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	listU, err := url.Parse(siteOrigin + listPath)
	require.NoError(t, err)
	jar.SetCookies(listU, []*http.Cookie{{Name: "session", Value: "stuck-term"}})

	c := &WorldArchitects{HTTPClient: &http.Client{
		Jar:       jar,
		Transport: recordHost(srv.URL, &mu, &urls, &agents, &cookies),
	}}
	jobs, err := c.FetchWithQuery(context.Background(), "catalog")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 3)

	require.Equal(t, []string{
		siteOrigin + listPath,
		siteOrigin + listPath + "/alpha",
		siteOrigin + listPath + "/beta",
		siteOrigin + listPath + "?page=2",
		siteOrigin + listPath + "/gamma",
		siteOrigin + listPath + "?page=3",
	}, urls)
	for _, raw := range urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.False(t, queryHasFold(u, "term"))
		require.NotContains(t, raw, "arhitect")
		require.NotContains(t, raw, "page=4")
		require.NotContains(t, raw, "page=9")
	}
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}
	for _, cookie := range cookies {
		require.Empty(t, cookie)
	}

	named := jobs[0]
	require.Equal(t, "Projektleiter Architektur", named.Title)
	require.Equal(t, "ffbk Architekten AG", named.Company)
	require.Equal(t, siteOrigin+listPath+"/alpha", named.URL)
	require.Empty(t, named.ApplyURL)
	require.Equal(t, "Planen und bauen im Büro.", named.Description)
	require.Empty(t, named.SalaryRaw)
	require.Empty(t, named.Location.Type)
	require.Equal(t, []string{"CH"}, named.Location.Countries)
	require.Equal(t, "Zürich", named.Location.Raw)
	require.Equal(t, time.Date(2026, 10, 14, 9, 41, 0, 0, time.UTC), named.PostedAt)
	require.Equal(t, domainutils.StableJobID(named.CompanyKey, named.Title), named.ID)
	require.NotContains(t, named.ID, named.URL)
	require.NotContains(t, named.Description, "Building of the Week")
	require.NotContains(t, named.Description, "90000")

	placeholder := jobs[1]
	require.Equal(t, "Architekt:in", placeholder.Title)
	require.Equal(t, "Confidential", placeholder.Company)
	require.Empty(t, placeholder.ApplyURL)
	require.Empty(t, placeholder.SalaryRaw)
	require.Empty(t, placeholder.Location.Type)
	require.Equal(t, []string{"CH"}, placeholder.Location.Countries)
	require.Equal(t, "Aarau", placeholder.Location.Raw)
	require.Equal(t, "Studio", placeholder.Description)
	require.Empty(t, placeholder.PostedAt)
	norm, err := domainutils.NormalizeListingURL(placeholder.URL)
	require.NoError(t, err)
	require.Contains(t, placeholder.ID, norm)
	require.NotEqual(t, domainutils.StableJobID(placeholder.CompanyKey, placeholder.Title), placeholder.ID)

	german := jobs[2]
	require.Equal(t, "Bauleiter", german.Title)
	require.Equal(t, "Schmelzle Partner", german.Company)
	require.Equal(t, "Freiraum in Hallwangen.", german.Description)
	require.Equal(t, []string{"DE"}, german.Location.Countries)
	require.Equal(t, "Hallwangen", german.Location.Raw)
	require.Empty(t, german.SalaryRaw)
	require.Empty(t, german.ApplyURL)
	require.Empty(t, german.Location.Type)
	require.Equal(t, time.Date(2026, 10, 12, 13, 28, 0, 0, time.UTC), german.PostedAt)
	require.NotContains(t, german.ID, german.URL)
	require.NotContains(t, german.ID, "IGNORE")
}

func TestFetchWithQuery_ignoresQueryAndSkipsTerm(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != listPath {
			t.Errorf("unexpected %s", r.URL.RequestURI())
			http.NotFound(w, r)
			return
		}
		if queryHasFold(r.URL, "term") {
			t.Errorf("term %s", r.URL.RawQuery)
			http.Error(w, "term", http.StatusBadRequest)
			return
		}
		switch r.URL.Query().Get("page") {
		case "", "2":
			_, _ = w.Write([]byte(`<form action="/en/jobs"><input name="term" value="arhitect"></form><a href="/en/jobs?term=arhitect">search</a><a href="/en/jobs?page=9">9</a>`))
		default:
			t.Errorf("page %s", r.URL.RawQuery)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &WorldArchitects{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil, nil)}}
	for _, query := range []string{"", "catalog", "term=foo", "Architekt"} {
		jobs, err := c.FetchWithQuery(context.Background(), query)
		require.NoError(t, err)
		require.Empty(t, jobs)
	}
	fetched, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, fetched)
	require.Len(t, urls, 10)
	for i, raw := range urls {
		if i%2 == 0 {
			require.Equal(t, siteOrigin+listPath, raw)
		} else {
			require.Equal(t, siteOrigin+listPath+"?page=2", raw)
		}
		require.NotContains(t, raw, "term=")
		require.NotContains(t, raw, "Architekt")
		require.NotContains(t, raw, "page=9")
	}
}

func TestFetch_emptyTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == listPath && r.URL.Query().Get("page") == "":
			_, _ = w.Write([]byte(`<li class="row"><a href="/en/jobs/empty">x</a></li>`))
		case r.URL.Path == listPath+"/empty":
			_, _ = w.Write([]byte(`<h1>  </h1><script type="application/ld+json">{"@type":"JobPosting","title":"Real"}</script>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &WorldArchitects{HTTPClient: &http.Client{Transport: recordHost(srv.URL, nil, nil, nil, nil)}}
	_, err := c.Fetch(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "empty title")
}

func TestGet_refusesTerm(t *testing.T) {
	for _, raw := range []string{
		siteOrigin + listPath + "?term=arhitect",
		siteOrigin + listPath + "?page=2&term=Architekt",
		siteOrigin + listPath + "?TERM=foo",
	} {
		_, err := get(context.Background(), http.DefaultClient, raw)
		require.ErrorContains(t, err, "refused")
	}
}

func queryHasFold(u *url.URL, key string) bool {
	for name := range u.Query() {
		if strings.EqualFold(name, key) {
			return true
		}
	}
	return false
}

const listPage1 = `
<form action="/en/jobs" method="post"><input name="term" value="arhitect"></form>
<a href="/en/jobs?term=arhitect">search</a>
<div class="well well-shadow row">
  <dl class="no-margin col-md-7">
    <dt><a href="http://world-architects.com/en/jobs/alpha?utm=1">Projektleiter</a></dt>
  </dl>
  <a class="btn" href="/en/jobs/alpha">View Top Job</a>
</div>
<ul>
<li class="row">
  <dl class="col-md-6">
    <dt><a href="/en/jobs/alpha">Projektleiter Architektur</a></dt>
    <dd><a href="/en/ffbk">ffbk Architekten AG</a></dd>
  </dl>
  <address>Zürich, Switzerland</address>
  <time datetime="10/14/26, 9:41 AM">14. October</time>
</li>
<li class="row">
  <dl>
    <dt><a href="https://www.world-architects.com/en/jobs/beta/">Architekt:in</a></dt>
    <dd>Confidential</dd>
  </dl>
  <address>Aarau, Switzerland</address>
</li>
</ul>
<div class="pagination">
  <a href="/en/jobs?page=2">2</a>
  <a href="/en/jobs?page=9">9</a>
</div>
`

const listPage2 = `
<li class="row">
  <a href="/en/jobs/gamma">Bauleiter</a>
  <time datetime="10/12/26, 1:28 PM">12. October</time>
</li>
<li class="row">
  <a href="/en/jobs/alpha">dup</a>
  <time datetime="10/14/26, 9:41 AM">14. October</time>
</li>
<a href="/en/jobs?page=4">4</a>
`

const listPage3 = `
<a href="/en/jobs/alpha">alpha</a>
<a href="/en/jobs/gamma">gamma</a>
<div class="pagination"><a href="/en/jobs?page=4">4</a></div>
`

const detailNamed = `
<script type="application/ld+json">{"@type":"JobPosting","title":"IGNORE","description":"nope","datePosted":"2020-01-01","baseSalary":90000}</script>
<div class="col-md-12">
  <h1>Projektleiter Architektur</h1>
  <h5><a href="/en/ffbk-architekten">ffbk Architekten AG <i class="wa-arrow"></i></a></h5>
</div>
<div class="col-md-7 m-b-3">
  <p>Planen und <b>bauen</b> im Büro.</p>
</div>
<dl>
  <dt>Provider</dt>
  <dd>ffbk Architekten AG<br>Basel<br>Switzerland<br>
    <a href="mailto:job@ffbk.example">mail</a>
    <a href="https://www.ffbk.example">www.ffbk.example</a>
  </dd>
  <dt>Job Location</dt>
  <dd>Zürich, Switzerland</dd>
  <dt>Date</dt>
  <dd><time datetime="1/1/20, 8:00 AM">14. October</time></dd>
</dl>
<div class="salary">90000 CHF</div>
<h2>Building of the Week</h2>
<p>Not the description</p>
`

const detailConfidential = `
<h1>Architekt:in</h1>
<h5>Confidential</h5>
<div class="col-md-7"><p>Studio</p></div>
<dl>
  <dt>Job Location</dt>
  <dd>Aarau, Switzerland</dd>
  <dt>Date</dt>
  <dd><time datetime="1/1/99, 8:00 AM">1. January</time></dd>
</dl>
<a href="https://apply.example/job">apply</a>
<a href="mailto:job@firm.example">mail</a>
`

const detailGermany = `
<script type="application/ld+json">{"title":"IGNORE","datePosted":"2020-01-02"}</script>
<h1>Bauleiter</h1>
<h5><a href="/en/schmelzle">Schmelzle Partner</a></h5>
<div class="col-md-7"><p>Freiraum in Hallwangen.</p></div>
<dl>
  <dt>Job Location</dt>
  <dd>Hallwangen, Germany</dd>
  <dt>Date</dt>
  <dd><time datetime="1/2/20, 3:04 AM">12. October</time></dd>
</dl>
<p><a href="https://jobs.example/apply">portal</a></p>
`

type recordTransport struct {
	baseURL string
	next    http.RoundTripper
	mu      *sync.Mutex
	urls    *[]string
	agents  *[]string
	cookies *[]string
}

func recordHost(base string, mu *sync.Mutex, urls *[]string, agents *[]string, cookies *[]string) recordTransport {
	if mu == nil {
		mu = &sync.Mutex{}
	}
	return recordTransport{baseURL: base, next: http.DefaultTransport, mu: mu, urls: urls, agents: agents, cookies: cookies}
}

func (r recordTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	if r.urls != nil {
		*r.urls = append(*r.urls, req.URL.String())
	}
	if r.agents != nil {
		*r.agents = append(*r.agents, req.Header.Get("User-Agent"))
	}
	if r.cookies != nil {
		*r.cookies = append(*r.cookies, req.Header.Get("Cookie"))
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
