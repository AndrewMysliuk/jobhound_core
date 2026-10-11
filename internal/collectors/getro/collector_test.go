package getro

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

//go:embed testdata/search_page.json
var searchPageJSON []byte

func TestTenants(t *testing.T) {
	require.Equal(t, []Tenant{
		{SourceID: "getro_accel", CollectionID: 8672},
		{SourceID: "getro_general_catalyst", CollectionID: 222},
		{SourceID: "getro_atomico", CollectionID: 36986},
		{SourceID: "getro_hv_capital", CollectionID: 234},
		{SourceID: "getro_point_nine", CollectionID: 1680},
		{SourceID: "getro_earlybird", CollectionID: 617},
		{SourceID: "getro_firstminute", CollectionID: 178},
		{SourceID: "getro_craft_ventures", CollectionID: 340},
	}, Tenants())
}

func TestFetch_doesNotDial(t *testing.T) {
	var calls int
	b := &Board{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("dial")
		})},
		Tenant: Tenants()[0],
	}
	jobs, err := b.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Zero(t, calls)
	require.Equal(t, "getro_accel", b.Name())
}

func TestFetchWithQuery_emptyDoesNotDial(t *testing.T) {
	var calls int
	b := &Board{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("dial")
		})},
		Tenant: Tenants()[0],
	}
	jobs, err := b.FetchWithQuery(context.Background(), "  ")
	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Zero(t, calls)
}

func TestFetchWithQuery_searchBodyAndMapping(t *testing.T) {
	var mu sync.Mutex
	var bodies [][]byte
	var paths []string
	var origins []string
	var cookies []string
	var accepts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		mu.Lock()
		bodies = append(bodies, raw)
		paths = append(paths, r.URL.Path)
		origins = append(origins, r.Header.Get("Origin"))
		cookies = append(cookies, r.Header.Get("Cookie"))
		accepts = append(accepts, r.Header.Get("Accept"))
		page := len(bodies) - 1
		mu.Unlock()
		require.Equal(t, http.MethodPost, r.Method)
		require.Empty(t, r.URL.RawQuery)
		if page == 0 {
			_, _ = w.Write(searchPageJSON)
			return
		}
		_, _ = w.Write([]byte(`{"results":{"jobs":[]}}`))
	}))
	t.Cleanup(srv.Close)

	b := &Board{
		HTTPClient: clientFor(srv),
		Countries:  testCountriesResolver(t),
		Tenant:     Tenants()[0],
	}
	jobs, err := b.FetchWithQuery(context.Background(), " vue ")
	require.NoError(t, err)
	require.Len(t, bodies, 2)
	require.Equal(t, []string{
		"/api/v2/collections/8672/search/jobs",
		"/api/v2/collections/8672/search/jobs",
	}, paths)
	for i, raw := range bodies {
		assertSearchBody(t, raw, "vue", i)
	}
	require.Equal(t, []string{"", ""}, origins)
	require.Equal(t, []string{"", ""}, cookies)
	require.Equal(t, []string{"application/json", "application/json"}, accepts)

	require.Len(t, jobs, 2)
	first := jobs[0]
	require.Equal(t, "getro_accel", first.Source)
	require.Equal(t, "Senior Frontend Engineer", first.Title)
	require.Equal(t, "Acme", first.Company)
	require.Empty(t, first.CompanyWebsite)
	require.Equal(t, "https://jobs.accel.com/companies/acme/jobs/1?gh_jid=99", first.URL)
	require.Equal(t, first.URL, first.ApplyURL)
	require.Equal(t, "Vue.js\nTypeScript", first.Description)
	require.Equal(t, []string{"Vue.js", "TypeScript"}, first.Tags)
	require.Equal(t, time.Unix(1710000000, 0).UTC(), first.PostedAt)
	require.Equal(t, "Berlin, Germany, United States", first.Location.Raw)
	require.Equal(t, []string{"DE", "US"}, first.Location.Countries)
	require.Equal(t, schema.LocationRemote, first.Location.Type)
	require.Equal(t, domainutils.CompanyKey("Acme"), first.CompanyKey)
	require.Equal(t, domainutils.StableJobID(first.CompanyKey, first.Title, first.Location.Raw), first.ID)

	second := jobs[1]
	require.Equal(t, "Backend Engineer", second.Title)
	require.Equal(t, "n8n", second.Company)
	require.Empty(t, second.CompanyWebsite)
	require.Equal(t, "https://jobs.accel.com/companies/n8n/jobs/2", second.URL)
	require.Equal(t, second.URL, second.ApplyURL)
	require.Equal(t, "Go", second.Description)
	require.True(t, second.PostedAt.IsZero())
	require.Equal(t, "Remote", second.Location.Raw)
	require.Empty(t, second.Location.Type)
}

func TestFetchWithQuery_queryWord(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "vue", want: "vue"},
		{in: " go ", want: "golang"},
		{in: "vue.js", want: "vue.js"},
		{in: "node vue", want: "node vue"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			var raw []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				raw, err = io.ReadAll(r.Body)
				require.NoError(t, err)
				_, _ = w.Write([]byte(`{"results":{"jobs":[]}}`))
			}))
			t.Cleanup(srv.Close)

			b := &Board{HTTPClient: clientFor(srv), Tenant: Tenants()[0]}
			jobs, err := b.FetchWithQuery(context.Background(), tt.in)
			require.NoError(t, err)
			require.Empty(t, jobs)
			assertSearchBody(t, raw, tt.want, 0)
		})
	}
}

func TestFetchWithQuery_duplicatePageNotAppended(t *testing.T) {
	page := []byte(`{"results":{"jobs":[{"title":"Engineer","url":"https://jobs.example/1?gh_jid=1","organization":{"name":"Acme"},"skills":["Vue.js"]}]}}`)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write(page)
	}))
	t.Cleanup(srv.Close)

	b := &Board{HTTPClient: clientFor(srv), Tenant: Tenants()[0], MaxPages: 5}
	jobs, err := b.FetchWithQuery(context.Background(), "vue")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, 2, calls)
	require.Equal(t, "https://jobs.example/1?gh_jid=1", jobs[0].URL)
	require.Empty(t, jobs[0].CompanyWebsite)
}

func TestFetchWithQuery_maxPages(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body := `{"results":{"jobs":[{"title":"Engineer","url":"https://jobs.example/` + strconv.Itoa(calls) + `","organization":{"name":"Acme"}}]}}`
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	b := &Board{HTTPClient: clientFor(srv), Tenant: Tenants()[0], MaxPages: 1}
	jobs, err := b.FetchWithQuery(context.Background(), "vue")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, 1, calls)
}

func TestFetchWithQuery_status(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		tenant     bool
		wantSubstr string
	}{
		{name: "404", status: http.StatusNotFound, tenant: true},
		{name: "410", status: http.StatusGone, tenant: true},
		{name: "429", status: http.StatusTooManyRequests, wantSubstr: "429"},
		{name: "missing jobs", status: http.StatusOK, body: `{"results":{"count":1}}`},
		{name: "null jobs", status: http.StatusOK, body: `{"results":{"jobs":null}}`},
		{name: "no results", status: http.StatusOK, body: `{"ok":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			b := &Board{HTTPClient: clientFor(srv), Tenant: Tenants()[0]}
			jobs, err := b.FetchWithQuery(context.Background(), "vue")
			require.Error(t, err)
			require.Nil(t, jobs)
			if tt.tenant {
				require.ErrorIs(t, err, collectors.ErrTenantUnavailable)
				require.NotContains(t, err.Error(), "429")
				return
			}
			require.NotErrorIs(t, err, collectors.ErrTenantUnavailable)
			if tt.wantSubstr != "" {
				require.Contains(t, err.Error(), tt.wantSubstr)
			}
		})
	}
}

func assertSearchBody(t *testing.T, raw []byte, query string, page int) {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, query, payload["query"])
	require.Equal(t, float64(20), payload["hitsPerPage"])
	require.Equal(t, float64(page), payload["page"])
	filters, ok := payload["filters"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{"Europe", "United States"}, filters["searchable_locations"])
	require.Equal(t, []any{"remote"}, filters["searchable_location_option"])
	_, hasQ := filters["q"]
	require.False(t, hasQ)
	_, hasLocations := filters["locations"]
	require.False(t, hasLocations)
}

func testCountriesResolver(t *testing.T) *utils.CountryResolver {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	f, err := os.Open(filepath.Join(repoRoot, "data", "countries.json"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	r, err := utils.LoadCountryResolver(f)
	require.NoError(t, err)
	return r
}

func clientFor(srv *httptest.Server) *http.Client {
	base := srv.Client().Transport
	if base == nil {
		base = http.DefaultTransport
	}
	return &http.Client{Transport: rewriteHost{baseURL: srv.URL, next: base}}
}

type rewriteHost struct {
	baseURL string
	next    http.RoundTripper
}

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	u, err := url.Parse(r.baseURL)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.URL.Scheme = u.Scheme
	clone.URL.Host = u.Host
	clone.Host = u.Host
	return r.next.RoundTrip(clone)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
