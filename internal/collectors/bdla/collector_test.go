package bdla

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

func TestFetch_listLinesAndDetail(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		switch {
		case r.URL.Path == listPath && r.URL.Query().Get("details") == "":
			if _, ok := r.URL.Query()["bl"]; ok || queryHasFold(r.URL, "toj") {
				t.Errorf("filter %s", r.URL.RawQuery)
				http.Error(w, "filter", http.StatusBadRequest)
				return
			}
			if r.URL.Query().Get("view") != "ccbk" {
				t.Errorf("view %s", r.URL.RawQuery)
				http.Error(w, "view", http.StatusBadRequest)
				return
			}
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
		case r.URL.Path == listPath && r.URL.Query().Get("details") == "12522":
			_, _ = w.Write([]byte(detailNamed))
		case r.URL.Path == listPath && r.URL.Query().Get("details") == "12521":
			_, _ = w.Write([]byte(detailConfidential))
		case r.URL.Path == listPath && r.URL.Query().Get("details") == "12520":
			_, _ = w.Write([]byte(detailJSONLD))
		default:
			t.Errorf("unexpected %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Bdla{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)}}
	jobs, err := c.FetchWithQuery(context.Background(), "catalog")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 3)

	require.Equal(t, []string{
		siteOrigin + listPath + "?view=ccbk",
		detailURL("12522"),
		detailURL("12521"),
		siteOrigin + listPath + "?view=ccbk&page=2",
		detailURL("12520"),
		siteOrigin + listPath + "?view=ccbk&page=3",
	}, urls)
	for _, raw := range urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.False(t, queryHasFold(u, "bl"))
		require.False(t, queryHasFold(u, "toj"))
		require.NotContains(t, raw, "99999")
		require.NotContains(t, raw, "88888")
		require.NotContains(t, raw, "77777")
	}
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}

	named := jobs[0]
	require.Equal(t, "Landschaftsarchitekt/in", named.Title)
	require.Equal(t, "von K GmbH", named.Company)
	require.Equal(t, detailURL("12522"), named.URL)
	require.Equal(t, "https://www.vonk.gmbh", named.ApplyURL)
	require.Equal(t, "Planen und bauen & leiten. hybrides Arbeiten.", named.Description)
	require.Empty(t, named.SalaryRaw)
	require.Empty(t, named.Location.Type)
	require.Empty(t, named.Location.Countries)
	require.Equal(t, "Ludwig-Jahn-Str. 60 73760 Ostfildern", named.Location.Raw)
	require.Equal(t, time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), named.PostedAt)
	require.Equal(t, domainutils.StableJobID(named.CompanyKey, named.Title), named.ID)
	require.NotContains(t, named.ID, named.URL)

	placeholder := jobs[1]
	require.Equal(t, "Architekt:in", placeholder.Title)
	require.Equal(t, "Confidential", placeholder.Company)
	require.Empty(t, placeholder.ApplyURL)
	require.Empty(t, placeholder.SalaryRaw)
	require.Empty(t, placeholder.Location.Type)
	require.Empty(t, placeholder.Location.Countries)
	require.Equal(t, "Zürich", placeholder.Location.Raw)
	require.Empty(t, placeholder.PostedAt)
	require.Equal(t, "Studio", placeholder.Description)
	norm, err := domainutils.NormalizeListingURL(placeholder.URL)
	require.NoError(t, err)
	require.Contains(t, placeholder.ID, norm)
	require.NotEqual(t, domainutils.StableJobID(placeholder.CompanyKey, placeholder.Title), placeholder.ID)

	ld := jobs[2]
	require.Equal(t, "Projektleitung", ld.Title)
	require.Equal(t, "Atelier Nord", ld.Company)
	require.Equal(t, "Freiraum planen.", ld.Description)
	require.Equal(t, "Berlin", ld.Location.Raw)
	require.Empty(t, ld.Location.Countries)
	require.Empty(t, ld.SalaryRaw)
	require.Equal(t, "https://jobs.example/apply", ld.ApplyURL)
	require.Empty(t, ld.Location.Type)
	require.Equal(t, time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), ld.PostedAt)
	require.NotContains(t, ld.ID, "IGNORE")
	require.NotContains(t, ld.ID, ld.URL)
}

func TestFetchWithQuery_ignoresQueryAndSkipsFilters(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != listPath || r.URL.Query().Get("details") != "" {
			t.Errorf("unexpected %s", r.URL.RequestURI())
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("view") != "ccbk" || r.URL.Query().Get("page") != "" {
			t.Errorf("query %s", r.URL.RawQuery)
			http.Error(w, "query", http.StatusBadRequest)
			return
		}
		if queryHasFold(r.URL, "bl") || queryHasFold(r.URL, "toj") {
			t.Errorf("filter %s", r.URL.RawQuery)
			http.Error(w, "filter", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`<a href="/de/jobs-praktika/jobangebote?bl=international">intl</a><a href="/de/jobs-praktika/jobangebote?toj=1">typ</a><script>details=99999</script>`))
	}))
	t.Cleanup(srv.Close)

	c := &Bdla{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}}
	for _, query := range []string{"", "catalog", "bl=international", "toj=1", "Architekt"} {
		jobs, err := c.FetchWithQuery(context.Background(), query)
		require.NoError(t, err)
		require.Empty(t, jobs)
	}
	fetched, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, fetched)
	require.Len(t, urls, 6)
	for _, raw := range urls {
		require.Equal(t, siteOrigin+listPath+"?view=ccbk", raw)
		require.NotContains(t, raw, "bl=")
		require.NotContains(t, raw, "toj=")
		require.NotContains(t, raw, "page=")
		require.NotContains(t, raw, "Architekt")
		require.NotContains(t, raw, "international")
	}
}

func TestFetch_emptyTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == listPath && r.URL.Query().Get("details") == "":
			_, _ = w.Write([]byte(`<div class="listLine"><a href="/de/jobs-praktika/jobangebote?details=9">x</a></div>`))
		case r.URL.Query().Get("details") == "9":
			_, _ = w.Write([]byte(`<h1>  </h1><script type="application/ld+json">{"@type":"JobPosting","title":"Real"}</script>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Bdla{HTTPClient: &http.Client{Transport: recordHost(srv.URL, nil, nil, nil)}}
	_, err := c.Fetch(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "empty title")
}

func TestGet_refusesFilters(t *testing.T) {
	for _, raw := range []string{
		siteOrigin + listPath + "?bl=international",
		siteOrigin + listPath + "?toj=1",
		siteOrigin + listPath + "?view=ccbk&bl=Bayern",
		siteOrigin + listPath + "?TOJ=4",
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
<script>
bldainnermap.bindPopup('<a href="/de/jobs-praktika/jobangebote?details=99999">von K GmbH</a>');
</script>
<a href="/de/jobs-praktika/jobangebote?bl=international">intl</a>
<a href="/de/jobs-praktika/jobangebote?bl=Bayern">bayern</a>
<a href="/de/jobs-praktika/jobangebote?toj=1">typ</a>
<a href="/de/jobs-praktika/jobangebote?details=88888">outside</a>
<div class="listLine bgp56">
  <h3><a href="/de/jobs-praktika/jobangebote?details=12522">Landschaftsarchitekt/in</a></h3>
  <a href="/de/jobs-praktika/jobangebote?details=12522&amp;utm=1">dup</a>
</div>
<div class="listLine">
  <a href="https://www.bdla.de/de/jobs-praktika/jobangebote?details=12521">Architekt:in</a>
</div>
<div class="pagination"><a href="/de/jobs-praktika/jobangebote?view=ccbk&amp;page=2">2</a></div>
`

const listPage2 = `
<script>details=77777</script>
<a href="/de/jobs-praktika/jobangebote?toj=2">typ</a>
<div class="listLine">
  <a href="/de/jobs-praktika/jobangebote?details=12520">Projektleitung</a>
</div>
<div class="listLine">
  <a href="/de/jobs-praktika/jobangebote?details=12522">dup page</a>
</div>
<div class="listLine"></div>
`

const listPage3 = `
<script>bindPopup('details=66666')</script>
<a href="/de/jobs-praktika/jobangebote?bl=Berlin">berlin</a>
<a href="/de/jobs-praktika/jobangebote?details=55555">map only</a>
<div class="pagination"><a href="/de/jobs-praktika/jobangebote?view=ccbk&amp;page=4">4</a></div>
`

const detailNamed = `
<div id="introAbove">
  <p>Angebot vom:  08.10.2026</p>
  <h1 class="bgp48 ugp8">Landschaftsarchitekt/in</h1>
  <div class="row">
    <div class="col-md-6">
      <div class="row">
        <div class="col-3"><p><strong>Firma</strong></p></div>
        <div class="col-9"><p>von K GmbH</p></div>
      </div>
      <div class="row">
        <div class="col-3"><p><strong>Einsatzort</strong></p></div>
        <div class="col-9"><p>Ludwig-Jahn-Str. 60<br />73760&nbsp;Ostfildern</p></div>
      </div>
    </div>
    <div class="col-md-6">
      <div class="row">
        <div class="col-3"><p><strong>Kontakt</strong></p></div>
        <div class="col-9"><p><a href="mailto: jobs@vonk.gmbh">E-Mail</a></p></div>
      </div>
    </div>
  </div>
</div>
<h3>Stellenbeschreibung</h3>
<p class="ugp8">Planen und <b>bauen</b> &amp; leiten. hybrides Arbeiten.</p>
<p class="jobdesc"></p>
<h3>Kontakt &amp; Ansprechpartner:in</h3>
<p><a href="mailto: jobs@vonk.gmbh">jobs@vonk.gmbh</a></p>
<p><a href="https://www.vonk.gmbh" target="_blank">www.vonk.gmbh</a></p>
<p><a href="/de/jobs-praktika/jobangebote?view=ccbk">zurück zur Liste</a></p>
<h1 class="text-center">bdla Newsletter</h1>
<p>Deutschland</p>
`

const detailConfidential = `
<div id="introAbove">
  <h1>Architekt:in</h1>
  <div class="row">
    <div class="col-3"><p><strong>Firma</strong></p></div>
    <div class="col-9"><p>Confidential</p></div>
  </div>
  <div class="row">
    <div class="col-3"><p><strong>Einsatzort</strong></p></div>
    <div class="col-9"><p>Zürich</p></div>
  </div>
</div>
<h3>Stellenbeschreibung</h3>
<p>Studio</p>
<h3>Kontakt</h3>
<p><a href="mailto:job@firm.example">job@firm.example</a></p>
`

const detailJSONLD = `
<script type="application/ld+json">{"@type":"JobPosting","title":"IGNORE","description":"nope","datePosted":"2020-01-01","baseSalary":42000,"jobLocation":{"address":{"addressCountry":"CH","addressLocality":"Zürich"}}}</script>
<p>Angebot vom: 22.09.2026</p>
<h1>Projektleitung</h1>
<div class="row">
  <div class="col-3"><p><strong>Firma</strong></p></div>
  <div class="col-9"><p>Atelier Nord</p></div>
</div>
<div class="row">
  <div class="col-3"><p><strong>Einsatzort</strong></p></div>
  <div class="col-9"><p>Berlin</p></div>
</div>
<h3>Stellenbeschreibung</h3>
<p>Freiraum planen.</p>
<h3>Kontakt &amp; Ansprechpartner:in</h3>
<p><a href="mailto:job@atelier.example">mail</a> <a href="https://jobs.example/apply">portal</a></p>
<p>Deutschland</p>
`

type recordTransport struct {
	baseURL string
	next    http.RoundTripper
	mu      *sync.Mutex
	urls    *[]string
	agents  *[]string
}

func recordHost(base string, mu *sync.Mutex, urls *[]string, agents *[]string) recordTransport {
	if mu == nil {
		mu = &sync.Mutex{}
	}
	return recordTransport{baseURL: base, next: http.DefaultTransport, mu: mu, urls: urls, agents: agents}
}

func (r recordTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if r.urls != nil {
		r.mu.Lock()
		*r.urls = append(*r.urls, req.URL.String())
		if r.agents != nil {
			*r.agents = append(*r.agents, req.Header.Get("User-Agent"))
		}
		r.mu.Unlock()
	}
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
