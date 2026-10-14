package baunetz

import (
	"context"
	"encoding/json"
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

func TestFetch_listLinksAndDetail(t *testing.T) {
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
		case "/stellenmarkt/", "/stellenmarkt":
			if r.URL.RawQuery != "" {
				t.Errorf("query %s", r.URL.RawQuery)
				http.Error(w, "query", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(listHTML))
		case "/stellenmarkt/job/architekt-mwd/73186":
			_, _ = w.Write([]byte(detailStateHTML(t, map[string]any{
				"title":               "Architekt (m/w/d)",
				"location":            "Köln",
				"company":             map[string]any{"title": "caspar."},
				"publicationDatetime": "2026-10-13T14:11:52.637999+02:00",
				"textList": []map[string]any{{
					"title": "Stellenbeschreibung",
					"text":  "<div>Planen und <b>bauen</b> &amp; leiten.</div>",
				}},
				"contactAddress": map[string]any{
					"country":         "Deutschland",
					"email":           "job@caspar.archi",
					"applicantPortal": "https://www.studiocaspar.com/karriere/architekt/",
				},
				"baseSalary": 0,
			}, map[string]any{
				"@type": "JobPosting",
				"title": "IGNORE",
			})))
		case "/stellenmarkt/job/vertraulich/2":
			_, _ = w.Write([]byte(detailStateHTML(t, map[string]any{
				"title":    "Architekt:in",
				"location": "Wien",
				"company":  map[string]any{"title": "Confidential"},
				"textList": []map[string]any{{"text": "<p>Studio</p>"}},
				"contactAddress": map[string]any{
					"country":         "Österreich",
					"email":           "job@firm.example",
					"applicantPortal": "bewerbung@firm.example",
				},
				"baseSalary": 42000,
			}, nil)))
		case "/stellenmarkt/job/atelier/3":
			_, _ = w.Write([]byte(`<script type="application/ld+json">` + mustJSON(t, map[string]any{
				"@context":    "https://schema.org",
				"@type":       "JobPosting",
				"title":       "Landschaftsarchitekt:in",
				"description": "<p>Zürich &amp; Holz</p>",
				"datePosted":  "2026-10-01T00:00:00Z",
				"hiringOrganization": map[string]any{
					"@type": "Organization",
					"name":  "Atelier Nord",
					"location": map[string]any{
						"@type":           "PostalAddress",
						"addressLocality": "Zürich",
						"addressCountry":  "Schweiz",
					},
				},
				"baseSalary": 0,
			}) + `</script>`))
		default:
			if strings.Contains(r.URL.Path, "suche") || strings.Contains(r.URL.Path, "/jobs/") {
				t.Errorf("forbidden %s", r.URL.RequestURI())
			}
			t.Errorf("unexpected %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Baunetz{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)}}
	jobs, err := c.FetchWithQuery(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 3)

	require.Equal(t, []string{
		listURL,
		siteOrigin + "/stellenmarkt/job/architekt-mwd/73186",
		siteOrigin + "/stellenmarkt/job/vertraulich/2",
		siteOrigin + "/stellenmarkt/job/atelier/3",
	}, urls)
	for _, raw := range urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		path := strings.TrimSuffix(strings.ToLower(u.Path), "/")
		require.NotEqual(t, "/suche", path)
		require.NotEqual(t, "/stellenmarkt/suche", path)
		require.NotContains(t, path, "/jobs/")
		require.Empty(t, u.RawQuery)
	}
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}

	named := jobs[0]
	require.Equal(t, "Architekt (m/w/d)", named.Title)
	require.Equal(t, "caspar.", named.Company)
	require.Equal(t, siteOrigin+"/stellenmarkt/job/architekt-mwd/73186", named.URL)
	require.Equal(t, "https://www.studiocaspar.com/karriere/architekt/", named.ApplyURL)
	require.Equal(t, "Planen und bauen & leiten.", named.Description)
	require.Empty(t, named.SalaryRaw)
	require.Empty(t, named.Location.Type)
	require.Empty(t, named.Location.Countries)
	require.Equal(t, "Köln", named.Location.Raw)
	require.Equal(t, time.Date(2026, 10, 13, 12, 11, 52, 637999000, time.UTC), named.PostedAt)
	require.Equal(t, domainutils.StableJobID(named.CompanyKey, named.Title), named.ID)
	require.NotContains(t, named.ID, named.URL)

	placeholder := jobs[1]
	require.Equal(t, "Wien", placeholder.Location.Raw)
	require.Empty(t, placeholder.Location.Countries)
	require.Empty(t, placeholder.ApplyURL)
	require.Equal(t, "42000", placeholder.SalaryRaw)
	require.Empty(t, placeholder.Location.Type)
	require.Empty(t, placeholder.PostedAt)
	require.Equal(t, "Studio", placeholder.Description)
	require.Contains(t, placeholder.ID, placeholder.URL)

	ld := jobs[2]
	require.Equal(t, "Landschaftsarchitekt:in", ld.Title)
	require.Equal(t, "Atelier Nord", ld.Company)
	require.Equal(t, "Zürich & Holz", ld.Description)
	require.Equal(t, "Zürich", ld.Location.Raw)
	require.Empty(t, ld.Location.Countries)
	require.Empty(t, ld.SalaryRaw)
	require.Empty(t, ld.ApplyURL)
	require.Empty(t, ld.Location.Type)
	require.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ld.PostedAt)
	require.NotContains(t, ld.ID, ld.URL)
}

func TestFetchWithQuery_ignoresQueryAndSkipsSearch(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stellenmarkt/" && r.URL.Path != "/stellenmarkt" {
			t.Errorf("unexpected %s", r.URL.RequestURI())
			http.NotFound(w, r)
			return
		}
		if r.URL.RawQuery != "" {
			t.Errorf("query %s", r.URL.RawQuery)
			http.Error(w, "query", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`<a href="/stellenmarkt/suche">Suche</a><a href="/suche">q</a><a href="/stellenmarkt/?page=2">next</a>`))
	}))
	t.Cleanup(srv.Close)

	c := &Baunetz{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}}
	for _, query := range []string{"", "catalog", "Architekt", "suche"} {
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
		require.NotContains(t, raw, "suche")
		require.NotContains(t, raw, "Architekt")
		require.NotContains(t, raw, "page=")
	}
}

func TestFetch_emptyTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stellenmarkt/", "/stellenmarkt":
			_, _ = w.Write([]byte(`<a href="/stellenmarkt/job/leer/9">x</a>`))
		case "/stellenmarkt/job/leer/9":
			_, _ = w.Write([]byte(detailStateHTML(t, map[string]any{
				"title":   "  ",
				"company": map[string]any{"title": "Büro"},
			}, map[string]any{
				"@type": "JobPosting",
				"title": "Real",
			})))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Baunetz{HTTPClient: &http.Client{Transport: recordHost(srv.URL, nil, nil, nil)}}
	_, err := c.Fetch(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "empty title")
}

const listHTML = `
<a href="/stellenmarkt/suche">Suche</a>
<a href="/suche?q=architekt">query</a>
<a href="/stellenmarkt/jobs/sweco-architects/566374">profile</a>
<a href="/stellenmarkt/?page=2">next</a>
<a href="/stellenmarkt/job/architekt-mwd/73186">A</a>
<a href="/stellenmarkt/job/architekt-mwd/73186?utm=1">A2</a>
<a href="https://www.baunetz.de/stellenmarkt/job/architekt-mwd/73186/">A3</a>
<a href="https://example.com/stellenmarkt/job/other/9">ext</a>
<a href="/stellenmarkt/job/vertraulich/2">B</a>
<a href="/stellenmarkt/job/atelier/3">C</a>
`

func detailStateHTML(t *testing.T, detail map[string]any, ld map[string]any) string {
	t.Helper()
	payload := map[string]any{
		"nonPersistentStore": map[string]any{
			"jobDetailsPageContent": detail,
		},
	}
	body := `<script>window.__INITIAL_STATE__=` + mustJSON(t, payload) + `;</script>`
	if ld != nil {
		body += `<script type="application/ld+json">` + mustJSON(t, ld) + `</script>`
	}
	return body
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

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
