package competitionline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetch_unfilteredPagesAndDetailJSON(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if _, ok := r.URL.Query()["fs"]; ok {
			t.Errorf("fs on %s", r.URL.RequestURI())
			http.Error(w, "fs", http.StatusBadRequest)
			return
		}
		if r.URL.Path == "/api/modgnikehtotsyek" {
			t.Errorf("forbidden path %s", r.URL.RequestURI())
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/jobs/":
			page := r.URL.Query().Get("page")
			if len(r.URL.Query()) != 1 || page == "" {
				t.Errorf("query %s", r.URL.RawQuery)
				http.Error(w, "query", http.StatusBadRequest)
				return
			}
			switch page {
			case "0":
				writeJSON(t, w, map[string]any{
					"total": 3,
					"next":  "/api/modgnikehtotsyek",
					"hits":  []map[string]any{{"id": 422600}, {"id": 2}, {"id": 422600}},
				})
			case "1":
				writeJSON(t, w, map[string]any{
					"total": 3,
					"next":  "",
					"hits":  []map[string]any{{"id": 3}},
				})
			default:
				t.Errorf("page %s", page)
				http.NotFound(w, r)
			}
		case "/api/jobs/422600":
			writeJSON(t, w, detailFixture{
				title: "Projektleiter:in", company: "h4a Architekten GmbH",
				description: "<div>Planen und <b>bauen</b> &amp; leiten.</div>",
				country:     "Deutschland", city: "Düsseldorf",
				posted:    "2026-10-13T14:11:52.637999+02:00",
				apply:     "https://greenbox.la/stellen/teamleitung/",
				applyNull: false,
			})
		case "/api/jobs/2":
			writeJSON(t, w, detailFixture{
				title: "Architekt:in", company: "Confidential",
				description: "<p>Studio</p>",
				country:     "Österreich", city: "Wien",
				apply: "bewerbung@firm.example",
			})
		case "/api/jobs/3":
			writeJSON(t, w, detailFixture{
				title: "Landschaftsarchitekt:in", company: "Atelier Nord",
				description: "Zürich",
				country:     "Schweiz", city: "Zürich",
				apply: "www.capattistaubach.de/karriere",
			})
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Competitionline{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)}}
	jobs, err := c.FetchWithQuery(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 3)

	require.Equal(t, []string{
		siteOrigin + "/api/jobs/?page=0",
		siteOrigin + "/api/jobs/422600",
		siteOrigin + "/api/jobs/2",
		siteOrigin + "/api/jobs/?page=1",
		siteOrigin + "/api/jobs/3",
	}, urls)
	for _, raw := range urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.NotContains(t, u.Path, "modgnikehtotsyek")
		_, hasFS := u.Query()["fs"]
		require.False(t, hasFS)
		require.NotContains(t, raw, "/de/jobs/")
	}
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}

	named := jobs[0]
	require.Equal(t, "Projektleiter:in", named.Title)
	require.Equal(t, "h4a Architekten GmbH", named.Company)
	require.Equal(t, siteOrigin+"/de/jobs/422600", named.URL)
	require.Equal(t, "https://greenbox.la/stellen/teamleitung/", named.ApplyURL)
	require.Equal(t, "Planen und bauen & leiten.", named.Description)
	require.Empty(t, named.SalaryRaw)
	require.Empty(t, named.Location.Type)
	require.Equal(t, []string{"DE"}, named.Location.Countries)
	require.Equal(t, "Düsseldorf", named.Location.Raw)
	require.Equal(t, time.Date(2026, 10, 13, 12, 11, 52, 637999000, time.UTC), named.PostedAt)
	require.Equal(t, domainutils.StableJobID(named.CompanyKey, named.Title), named.ID)
	require.NotContains(t, named.ID, named.URL)

	placeholder := jobs[1]
	require.Equal(t, []string{"AT"}, placeholder.Location.Countries)
	require.Equal(t, "Wien", placeholder.Location.Raw)
	require.Empty(t, placeholder.ApplyURL)
	require.Empty(t, placeholder.SalaryRaw)
	require.Empty(t, placeholder.Location.Type)
	require.Empty(t, placeholder.PostedAt)
	require.Contains(t, placeholder.ID, placeholder.URL)

	swiss := jobs[2]
	require.Equal(t, []string{"CH"}, swiss.Location.Countries)
	require.Equal(t, "Zürich", swiss.Location.Raw)
	require.Empty(t, swiss.ApplyURL)
	require.Empty(t, swiss.SalaryRaw)
	require.Empty(t, swiss.Location.Type)
	require.NotContains(t, swiss.ID, swiss.URL)
}

func TestFetchWithQuery_ignoresQueryAndStopsOnEmptyNext(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/jobs/" || r.URL.RawQuery != "page=0" {
			t.Errorf("unexpected %s", r.URL.RequestURI())
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, map[string]any{"total": 0, "next": nil, "hits": []any{}})
	}))
	t.Cleanup(srv.Close)

	c := &Competitionline{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}}
	for _, query := range []string{"", "catalog", "Architekt", "fs=architekt"} {
		jobs, err := c.FetchWithQuery(context.Background(), query)
		require.NoError(t, err)
		require.Empty(t, jobs)
	}
	fetched, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, fetched)
	require.Len(t, urls, 5)
	for _, raw := range urls {
		require.Equal(t, listURL(0), raw)
		require.NotContains(t, raw, "fs")
		require.NotContains(t, raw, "Architekt")
	}
}

func TestFetch_emptyTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/jobs/":
			writeJSON(t, w, map[string]any{
				"next": "",
				"hits": []map[string]any{{"id": 9}},
			})
		case "/api/jobs/9":
			writeJSON(t, w, map[string]any{"title": "  ", "company_name": "Büro"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Competitionline{HTTPClient: &http.Client{Transport: recordHost(srv.URL, nil, nil, nil)}}
	_, err := c.Fetch(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "empty title")
}

type detailFixture struct {
	title       string
	company     string
	description string
	country     string
	city        string
	posted      string
	apply       string
	applyNull   bool
}

func (f detailFixture) MarshalJSON() ([]byte, error) {
	row := map[string]any{
		"title":            f.title,
		"company_name":     f.company,
		"description":      f.description,
		"country":          f.country,
		"city":             f.city,
		"publication_date": f.posted,
	}
	if f.applyNull {
		row["directApply"] = nil
	} else {
		row["directApply"] = f.apply
	}
	return json.Marshal(row)
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(v))
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
