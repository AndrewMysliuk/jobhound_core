package workingnomads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

const searchFixtureJSON = `{
  "took": 4,
  "timed_out": false,
  "hits": {
    "total": { "value": 1, "relation": "eq" },
    "hits": [
      {
        "_index": "jobsapi",
        "_id": "1502763",
        "_score": 12.5,
        "_source": {
          "id": 1502763,
          "title": "Senior Full Stack Developer",
          "slug": "senior-full-stack-developer-acme-1502763",
          "company": "Acme Remote",
          "category_name": "Development",
          "description": "<p>Ship features end-to-end.</p>",
          "position_type": "ft",
          "tags": ["javascript", "react"],
          "locations": ["European Union"],
          "location_base": "EU",
          "pub_date": "2026-03-28T10:00:00Z",
          "apply_option": "with_your_ats",
          "apply_url": "https://example.com/apply",
          "expired": false,
          "salary_range": "€70k – €90k",
          "experience_level": "SENIOR_LEVEL"
        }
      }
    ]
  }
}`

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

func TestDecodeSearchJSON_toJob(t *testing.T) {
	var resp searchResponse
	require.NoError(t, json.Unmarshal([]byte(searchFixtureJSON), &resp))
	require.Len(t, resp.Hits.Hits, 1)
	j, err := jobFromSource(testCountriesResolver(t), resp.Hits.Hits[0].Source)
	require.NoError(t, err)
	require.Equal(t, SourceName, j.Source)
	require.Equal(t, "Senior Full Stack Developer", j.Title)
	require.Equal(t, "Acme Remote", j.Company)
	require.Equal(t, "https://www.workingnomads.com/jobs/senior-full-stack-developer-acme-1502763", j.URL)
	require.Equal(t, "https://example.com/apply", j.ApplyURL)
	wantPosted := time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC)
	require.True(t, j.PostedAt.Equal(wantPosted))
	require.Equal(t, "Ship features end-to-end.", j.Description)
	require.Equal(t, "€70k – €90k", j.SalaryRaw)
	require.Equal(t, []string{"javascript", "react"}, j.Tags)
	require.NotNil(t, j.Remote)
	require.False(t, *j.Remote)
	require.NotNil(t, j.Position)
	require.Equal(t, "full-stack", *j.Position)
	require.NotEmpty(t, j.ID)
	wantID, err := domainutils.StableJobID(SourceName, j.URL)
	require.NoError(t, err)
	require.Equal(t, wantID, j.ID)
}

func TestDecodeSearchJSON_skipsExpired(t *testing.T) {
	const both = `{
  "hits": {
    "total": { "value": 2, "relation": "eq" },
    "hits": [
      {
        "_source": {
          "title": "Expired",
          "slug": "expired-one",
          "company": "X",
          "description": "<p>a</p>",
          "pub_date": "2026-01-01T00:00:00Z",
          "apply_option": "with_your_ats",
          "expired": true
        }
      },
      {
        "_source": {
          "title": "Active",
          "slug": "active-one",
          "company": "Y",
          "description": "<p>b</p>",
          "pub_date": "2026-01-02T00:00:00Z",
          "apply_option": "with_your_ats",
          "expired": false
        }
      }
    ]
  }
}`
	var resp searchResponse
	require.NoError(t, json.Unmarshal([]byte(both), &resp))
	var jobs []schema.Job
	for _, h := range resp.Hits.Hits {
		j, err := jobFromSource(nil, h.Source)
		if errors.Is(err, errSkipHit) {
			continue
		}
		require.NoError(t, err)
		jobs = append(jobs, j)
	}
	require.Len(t, jobs, 1)
	require.Equal(t, "Active", jobs[0].Title)
}

func TestFetch_httptest(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/jobsapi/_search", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(b), `"from":0`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searchFixtureJSON))
	})
	coll := &WorkingNomads{
		HTTPClient: srv.Client(),
		SearchURL:  srv.URL + "/jobsapi/_search",
		PageSize:   100,
		Countries:  testCountriesResolver(t),
		MaxPages:   -1,
	}
	jobs, err := coll.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "Senior Full Stack Developer", jobs[0].Title)
}

func TestFetch_maxFetchJobs(t *testing.T) {
	hit := func(i int) map[string]any {
		return map[string]any{
			"_source": map[string]any{
				"title":        fmt.Sprintf("Role %d", i),
				"slug":         fmt.Sprintf("slug-%d-1502763", i),
				"company":      "Co",
				"description":  "<p>x</p>",
				"pub_date":     "2026-01-01T00:00:00Z",
				"apply_option": "with_your_ats",
				"expired":      false,
			},
		}
	}
	body, err := json.Marshal(map[string]any{
		"hits": map[string]any{
			"total": map[string]any{"value": 5, "relation": "eq"},
			"hits":  []map[string]any{hit(0), hit(1), hit(2), hit(3), hit(4)},
		},
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/jobsapi/_search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	coll := &WorkingNomads{
		HTTPClient:   srv.Client(),
		SearchURL:    srv.URL + "/jobsapi/_search",
		PageSize:     100,
		Countries:    testCountriesResolver(t),
		MaxFetchJobs: 2,
		MaxPages:     -1,
	}
	jobs, err := coll.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, "Role 0", jobs[0].Title)
	require.Equal(t, "Role 1", jobs[1].Title)
}

func TestFetch_defaultMaxPages_twoRequests(t *testing.T) {
	hit := func(i int) map[string]any {
		return map[string]any{
			"_source": map[string]any{
				"title":        fmt.Sprintf("Role %d", i),
				"slug":         fmt.Sprintf("slug-%d-1502763", i),
				"company":      "Co",
				"description":  "<p>x</p>",
				"pub_date":     "2026-01-01T00:00:00Z",
				"apply_option": "with_your_ats",
				"expired":      false,
			},
		}
	}
	var reqCount int
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/jobsapi/_search", func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		switch reqCount {
		case 1:
			require.Contains(t, string(b), `"from":0`)
			body, err := json.Marshal(map[string]any{
				"hits": map[string]any{
					"total": map[string]any{"value": 100, "relation": "eq"},
					"hits":  []map[string]any{hit(0), hit(1)},
				},
			})
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		case 2:
			require.Contains(t, string(b), `"from":2`)
			body, err := json.Marshal(map[string]any{
				"hits": map[string]any{
					"total": map[string]any{"value": 100, "relation": "eq"},
					"hits":  []map[string]any{hit(2), hit(3)},
				},
			})
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		default:
			t.Fatalf("unexpected third jobsapi request")
		}
	})
	coll := &WorkingNomads{
		HTTPClient: srv.Client(),
		SearchURL:  srv.URL + "/jobsapi/_search",
		PageSize:   2,
		Countries:  testCountriesResolver(t),
	}
	jobs, err := coll.Fetch(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, reqCount)
	require.Len(t, jobs, 4)
	require.Equal(t, "Role 0", jobs[0].Title)
	require.Equal(t, "Role 3", jobs[3].Title)
}
