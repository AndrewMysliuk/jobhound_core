package himalayas

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

//go:embed testdata/search_minimal.json
var searchMinimalJSON []byte

func testCountriesResolver(t *testing.T) *utils.CountryResolver {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	p := filepath.Join(repoRoot, "data", "countries.json")
	f, err := os.Open(p)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	r, err := utils.LoadCountryResolver(f)
	require.NoError(t, err)
	return r
}

func TestMap_searchMinimalFixture(t *testing.T) {
	var env apiEnvelope
	require.NoError(t, json.Unmarshal(searchMinimalJSON, &env))
	require.Len(t, env.Jobs, 1)

	cr := testCountriesResolver(t)
	j, err := jobFromWire(cr, env.Jobs[0], nil)
	require.NoError(t, err)

	require.Equal(t, SourceName, j.Source)
	require.Equal(t, "Software Engineer - Vue.JS - Offshore", j.Title)
	require.Equal(t, "Photon Interactive UK Limited", j.Company)
	wantURL, err := domainutils.NormalizeListingURL("https://himalayas.app/companies/photon-interactive-uk-limited/jobs/software-engineer-vue-js-offshore")
	require.NoError(t, err)
	require.Equal(t, wantURL, j.URL)
	require.Equal(t, time.Unix(1774053663, 0).UTC(), j.PostedAt)
	require.Equal(t, []float64{5.5}, j.TimezoneOffsets)
	require.Equal(t, "IN", j.CountryCode)
	require.NotNil(t, j.Remote)
	require.True(t, *j.Remote)
	require.NotEmpty(t, j.ID)
}

func TestBrowsePagination_stopConditions(t *testing.T) {
	t.Parallel()
	cr := testCountriesResolver(t)

	makeJob := func(i int) jobWire {
		u := fmt.Sprintf("https://h.test/jobs/%d", i)
		return jobWire{
			Title:       fmt.Sprintf("Role %d", i),
			CompanyName: "Co",
			GUID:        u,
			Description: "<p>x</p>",
			PubDate:     1700000000,
		}
	}

	t.Run("short second page", func(t *testing.T) {
		var calls int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			var jobs []jobWire
			switch off {
			case 0:
				for i := range 20 {
					jobs = append(jobs, makeJob(i))
				}
				_ = json.NewEncoder(w).Encode(apiEnvelope{Jobs: jobs, Offset: 0, Limit: 20, TotalCount: 25})
			default:
				for i := 20; i < 25; i++ {
					jobs = append(jobs, makeJob(i))
				}
				_ = json.NewEncoder(w).Encode(apiEnvelope{Jobs: jobs, Offset: off, Limit: 20, TotalCount: 25})
			}
		}))
		t.Cleanup(srv.Close)

		c := &Himalayas{
			HTTPClient: srv.Client(),
			BrowseURL:  srv.URL + "/api",
			Countries:  cr,
			MaxPages:   10,
		}
		jobs, err := c.Fetch(context.Background())
		require.NoError(t, err)
		require.Len(t, jobs, 25)
		require.Equal(t, 2, calls)
	})

	t.Run("max_pages stops browse early", func(t *testing.T) {
		var calls int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			var jobs []jobWire
			for i := range 20 {
				jobs = append(jobs, makeJob(calls*100+i))
			}
			_ = json.NewEncoder(w).Encode(apiEnvelope{Jobs: jobs, Limit: 20, TotalCount: 1000})
		}))
		t.Cleanup(srv.Close)

		c := &Himalayas{
			HTTPClient: srv.Client(),
			BrowseURL:  srv.URL + "/api",
			Countries:  cr,
			MaxPages:   1,
		}
		jobs, err := c.Fetch(context.Background())
		require.NoError(t, err)
		require.Len(t, jobs, 20)
		require.Equal(t, 1, calls)
	})
}

func TestSearchMode_onePage(t *testing.T) {
	cr := testCountriesResolver(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "1", r.URL.Query().Get("page"))
		require.Equal(t, "go", r.URL.Query().Get("q"))
		j := makeJob(1)
		j.Title = "Go engineer"
		_ = json.NewEncoder(w).Encode(apiEnvelope{Jobs: []jobWire{j}, Offset: 0, Limit: 20, TotalCount: 1})
	}))
	t.Cleanup(srv.Close)

	c := &Himalayas{
		HTTPClient:      srv.Client(),
		SearchURL:       srv.URL + "/search",
		Countries:       cr,
		UseSearch:       true,
		SearchQuery:     "go",
		SearchStartPage: 1,
		MaxPages:        3,
	}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "Go engineer", jobs[0].Title)
}

func TestHTTP429_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	c := &Himalayas{
		HTTPClient: srv.Client(),
		BrowseURL:  srv.URL + "/api",
		Countries:  testCountriesResolver(t),
		MaxPages:   1,
	}
	_, err := c.Fetch(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "429")
}

func makeJob(i int) jobWire {
	u := fmt.Sprintf("https://h.test/jobs/%d", i)
	return jobWire{
		Title:       fmt.Sprintf("T%d", i),
		CompanyName: "C",
		GUID:        u,
		Description: "<p>d</p>",
		PubDate:     1700000000,
	}
}
