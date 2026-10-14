package bestjobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetchWithQuery_suffixesPaginationAndMapping(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/locuri-de-munca/arhitect+proiectant-in-romania":
			writeHTML(t, w, listPageProps(
				"arhitect+proiectant-in-romania",
				"c-ro",
				[]string{"landscape"},
			))
		case "/locuri-de-munca/arhitect+proiectant-in-remote":
			writeHTML(t, w, listPageProps(
				"arhitect+proiectant-in-remote",
				"c-remote",
				[]string{"landscape", "hybrid-role"},
			))
		case "/api/proxy/v2/jobs":
			q := r.URL.Query()
			require.NotContains(t, q, "_lat")
			require.NotContains(t, q, "_lon")
			require.Equal(t, "24", q.Get("limit"))
			switch q.Get("cursor") {
			case "c-ro":
				require.Equal(t, "arhitect+proiectant-in-romania", q.Get("keyword"))
				require.Equal(t, "arhitect+proiectant-in-romania", q.Get("slug[]"))
				writeJSON(t, w, map[string]any{
					"items":      []map[string]string{{"slug": "office-role"}},
					"nextCursor": nil,
				})
			case "c-remote":
				require.Equal(t, "arhitect+proiectant-in-remote", q.Get("keyword"))
				require.Equal(t, "arhitect+proiectant-in-remote", q.Get("slug[]"))
				writeJSON(t, w, map[string]any{
					"items":      []map[string]string{{"slug": "later-remote"}},
					"nextCursor": "c-remote-2",
				})
			case "c-remote-2":
				require.Equal(t, "arhitect+proiectant-in-remote", q.Get("keyword"))
				writeJSON(t, w, map[string]any{
					"items":      []any{},
					"nextCursor": nil,
				})
			default:
				t.Errorf("unexpected cursor %q", q.Get("cursor"))
				http.NotFound(w, r)
			}
		case "/loc-de-munca/landscape":
			writeHTML(t, w, detailPage(detailFixture{
				title: "Junior Landscape Designer", company: "Riviera Gardens",
				description: "<p>Studio <b>work</b></p>", remote: true,
				estimated: "960 - 1060", apply: "jobs@firm.ro",
			}))
		case "/loc-de-munca/office-role":
			writeHTML(t, w, detailPage(detailFixture{
				title: "Inginer proiectant", company: "DEConstruct AG SRL",
				description: "<p>Office</p>", salary: "900 - 1000 €/luna",
				estimated: "1555 - 1720", apply: "https://firm.example/apply?ref=1",
				locations: []map[string]string{{
					"name": "Cluj-Napoca", "tooltip": "Cluj-Napoca, România",
				}},
			}))
		case "/loc-de-munca/hybrid-role":
			writeHTML(t, w, detailPage(detailFixture{
				title: "Arhitect", partial: true, apply: "firm.ro",
			}))
		case "/loc-de-munca/later-remote":
			writeHTML(t, w, detailPage(detailFixture{
				title: "Peisagist", company: "Studio", remote: true,
			}))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &BestJobs{
		HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)},
		Countries:  testCountries(t),
	}
	jobs, err := c.FetchWithQuery(context.Background(), " arhitect   proiectant ")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())

	var lists []string
	var apis []string
	for _, raw := range urls {
		require.NotContains(t, raw, "?page=")
		require.NotEqual(t, "https://www.bestjobs.eu/locuri-de-munca/arhitect+proiectant", raw)
		require.NotContains(t, raw, "_lat=")
		require.NotContains(t, raw, "_lon=")
		switch {
		case strings.Contains(raw, "/locuri-de-munca/"):
			lists = append(lists, raw)
		case strings.Contains(raw, "/api/proxy/v2/jobs?"):
			apis = append(apis, raw)
		}
	}
	require.Equal(t, []string{
		"https://www.bestjobs.eu/locuri-de-munca/arhitect+proiectant-in-romania",
		"https://www.bestjobs.eu/locuri-de-munca/arhitect+proiectant-in-remote",
	}, lists)
	require.Len(t, apis, 3)
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}

	require.Len(t, jobs, 4)

	remote := jobs[0]
	require.Equal(t, "Junior Landscape Designer", remote.Title)
	require.Equal(t, "Riviera Gardens", remote.Company)
	require.Equal(t, "https://www.bestjobs.eu/loc-de-munca/landscape", remote.URL)
	require.Empty(t, remote.ApplyURL)
	require.Equal(t, "Studio work", remote.Description)
	require.True(t, remote.PostedAt.IsZero())
	require.Empty(t, remote.SalaryRaw)
	require.NotContains(t, remote.SalaryRaw, "960")
	require.Equal(t, schema.LocationRemote, remote.Location.Type)
	require.Equal(t, domainutils.StableJobID(remote.CompanyKey, remote.Title), remote.ID)

	office := jobs[1]
	require.Equal(t, "Inginer proiectant", office.Title)
	require.Equal(t, "https://firm.example/apply?ref=1", office.ApplyURL)
	require.Equal(t, "900 - 1000 €/luna", office.SalaryRaw)
	require.NotContains(t, office.SalaryRaw, "1555")
	require.True(t, office.PostedAt.IsZero())
	require.Equal(t, schema.LocationOffice, office.Location.Type)
	require.Equal(t, []string{"RO"}, office.Location.Countries)
	require.Equal(t, "Cluj-Napoca, România", office.Location.Raw)
	require.Equal(t, domainutils.CompanyKey("DEConstruct AG SRL"), office.CompanyKey)
	require.Equal(t, domainutils.StableJobID(office.CompanyKey, office.Title), office.ID)
	require.NotContains(t, office.ID, office.URL)

	hybrid := jobs[2]
	require.Equal(t, schema.LocationHybrid, hybrid.Location.Type)
	require.Empty(t, hybrid.ApplyURL)
	require.Empty(t, hybrid.Company)
	require.Contains(t, hybrid.ID, "https://www.bestjobs.eu/loc-de-munca/hybrid-role")

	require.Equal(t, "Peisagist", jobs[3].Title)
}

func TestFetchWithQuery_missingFiltersSkipAPI(t *testing.T) {
	var urls []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/locuri-de-munca/arhitect-in-romania", "/locuri-de-munca/arhitect-in-remote":
			writeHTML(t, w, map[string]any{
				"infiniteQueryParams": map[string]any{
					"objFilterParams": map[string]string{"_lat": "45.1", "_lon": "25.1"},
					"fetchUrlParams":  "?limit=24&_lat=45.1&_lon=25.1",
				},
				"jobListCardsFromServer": map[string]any{
					"items":      []map[string]string{{"slug": "only-card"}},
					"nextCursor": "should-not-fetch",
				},
			})
		case "/loc-de-munca/only-card":
			writeHTML(t, w, detailPage(detailFixture{title: "Arhitect", company: "Firm"}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &BestJobs{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}}
	jobs, err := c.FetchWithQuery(context.Background(), "arhitect")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	for _, raw := range urls {
		require.NotContains(t, raw, "/api/proxy/v2/jobs")
		require.NotContains(t, raw, "keyword=")
	}
}

func TestFetch_emptyQueryDoesNotDial(t *testing.T) {
	calls := 0
	c := &BestJobs{HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, http.ErrServerClosed
	})}}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, jobs)
	again, err := c.FetchWithQuery(context.Background(), "  ")
	require.NoError(t, err)
	require.Empty(t, again)
	require.Zero(t, calls)
}

type detailFixture struct {
	title       string
	company     string
	description string
	salary      string
	estimated   string
	apply       string
	remote      bool
	partial     bool
	locations   []map[string]string
}

func detailPage(f detailFixture) map[string]any {
	job := map[string]any{
		"title":           f.title,
		"description":     f.description,
		"salary":          nil,
		"estimatedSalary": f.estimated,
		"remote":          f.remote,
		"partialRemote":   f.partial,
		"ownApplyUrl":     nil,
		"employer":        map[string]any{"employerName": f.company},
		"locations":       f.locations,
		"jobId":           52071248,
	}
	if f.salary != "" {
		job["salary"] = f.salary
	}
	if f.apply != "" {
		job["ownApplyUrl"] = f.apply
	}
	return map[string]any{"job": job}
}

func listPageProps(keyword, cursor string, slugs []string) map[string]any {
	items := make([]map[string]string, len(slugs))
	for i, slug := range slugs {
		items[i] = map[string]string{"slug": slug}
	}
	escaped := url.QueryEscape(keyword)
	return map[string]any{
		"infiniteQueryParams": map[string]any{
			"objFilterParams": map[string]string{
				"keyword": keyword,
				"_lat":    "45.642680",
				"_lon":    "25.588725",
			},
			"fetchUrlParams": "?limit=10&locale=ro&slug%5B%5D=" + escaped + "&keyword=" + escaped + "&_lat=45.642680&_lon=25.588725",
		},
		"jobListCardsFromServer": map[string]any{
			"items":      items,
			"nextCursor": cursor,
		},
	}
}

func writeHTML(t *testing.T, w http.ResponseWriter, pageProps any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"props": map[string]any{"pageProps": pageProps},
	})
	require.NoError(t, err)
	_, err = w.Write(append(append([]byte(`<html><script id="__NEXT_DATA__" type="application/json">`), raw...), []byte(`</script></html>`)...))
	require.NoError(t, err)
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func testCountries(t *testing.T) *utils.CountryResolver {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	f, err := os.Open(filepath.Join(repoRoot, "data", "countries.json"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	resolver, err := utils.LoadCountryResolver(f)
	require.NoError(t, err)
	return resolver
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
