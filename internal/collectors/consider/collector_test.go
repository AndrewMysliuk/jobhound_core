package consider

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

//go:embed testdata/search_page.json
var searchPageJSON []byte

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

func TestFetchUnion_fixture(t *testing.T) {
	t.Parallel()

	var posts [][]byte
	var csrfHeaders []string
	var cookies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/jobs":
			require.Equal(t, http.MethodGet, r.Method)
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "sess-1", Path: "/"})
			_, _ = io.WriteString(w, `<html><script>{"csrfToken":"csrf-1"}</script></html>`)
		case "/api-boards/search-jobs":
			require.Equal(t, http.MethodPost, r.Method)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			posts = append(posts, body)
			csrfHeaders = append(csrfHeaders, r.Header.Get("x-csrf-token"))
			cookies = append(cookies, r.Header.Get("Cookie"))
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			if len(posts) == 1 {
				_, _ = w.Write(searchPageJSON)
				return
			}
			_, _ = io.WriteString(w, `{"jobs":[]}`)
		default:
			http.Error(w, "unexpected path", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)

	base := srv.Client()
	require.Nil(t, base.Jar)
	board := &Board{
		HTTPClient: base,
		Countries:  testCountriesResolver(t),
		Tenant: Tenant{
			SourceID: "consider_andreessen_horowitz",
			Host:     srv.URL,
			BoardID:  "andreessen-horowitz",
		},
		MaxPages: 5,
	}
	jobs, err := board.FetchUnion(context.Background(), []string{
		" vue ", "golang", "typescript", "react", "node", "python", "go",
	})
	require.NoError(t, err)
	require.Nil(t, base.Jar)
	require.Len(t, posts, 2)
	require.Equal(t, []string{"csrf-1", "csrf-1"}, csrfHeaders)
	require.Equal(t, []string{"session=sess-1", "session=sess-1"}, cookies)

	var first searchBody
	require.NoError(t, json.Unmarshal(posts[0], &first))
	require.NotContains(t, string(posts[0]), "remoteOnly")
	require.False(t, first.Grouped)
	require.Equal(t, "andreessen-horowitz", first.Board.ID)
	require.True(t, first.Board.IsParent)
	require.Equal(t, []string{"Europe", "United States"}, first.Query.Locations)
	require.Equal(t, []string{
		"resume:Vue.js",
		"resume:Go",
		"resume:TypeScript",
		"resume:React.js",
		"resume:Node.js",
	}, first.Query.Skills)
	require.NotContains(t, first.Query.Skills, "resume:React Native")
	require.NotContains(t, first.Query.Skills, "resume:Node")

	var firstMeta struct {
		Size     int    `json:"size"`
		Sequence string `json:"sequence"`
	}
	require.NoError(t, json.Unmarshal(first.Meta, &firstMeta))
	require.Equal(t, 50, firstMeta.Size)
	require.Empty(t, firstMeta.Sequence)

	var second searchBody
	require.NoError(t, json.Unmarshal(posts[1], &second))
	require.Equal(t, first.Query, second.Query)
	require.Equal(t, first.Board, second.Board)
	var secondMeta struct {
		Size     int    `json:"size"`
		Sequence string `json:"sequence"`
	}
	require.NoError(t, json.Unmarshal(second.Meta, &secondMeta))
	require.Equal(t, 50, secondMeta.Size)
	require.Equal(t, "cursor-1", secondMeta.Sequence)

	require.Len(t, jobs, 1)
	job := jobs[0]
	require.Equal(t, "consider_andreessen_horowitz", job.Source)
	require.Equal(t, "Software Engineer", job.Title)
	require.Equal(t, "Databricks", job.Company)
	require.Equal(t, "https://databricks.com", job.CompanyWebsite)
	require.Equal(t, "https://boards.greenhouse.io/databricks/jobs/123?gh_jid=123", job.URL)
	require.Equal(t, "https://boards.greenhouse.io/databricks/jobs/123?gh_jid=123&utm_source=consider", job.ApplyURL)
	require.Equal(t, "Go\ngolang\nVue.js", job.Description)
	require.Equal(t, []string{"Go", "Vue.js"}, job.Tags)
	require.Equal(t, time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC), job.PostedAt)
	require.Equal(t, "120000-180000 USD", job.SalaryRaw)
	require.Equal(t, "Berlin, Germany", job.Location.Raw)
	require.Equal(t, schema.LocationRemote, job.Location.Type)
	require.Equal(t, []string{"DE", "US"}, job.Location.Countries)
	require.Equal(t, []string{schema.RegionCodeEurope.String()}, job.Location.Regions)
	require.NotEmpty(t, job.ID)
	require.Equal(t, board.Name(), job.Source)
}

func TestFetchUnion_unknownQuery(t *testing.T) {
	t.Parallel()

	t.Run("dropped from the skill list", func(t *testing.T) {
		t.Parallel()
		var skills []string
		var calls int
		board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs" {
				_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
				return
			}
			calls++
			var body searchBody
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			skills = body.Query.Skills
			_, _ = io.WriteString(w, `{"jobs":[]}`)
		})
		jobs, err := board.FetchUnion(context.Background(), []string{"python", "vue", "go", "Vue"})
		require.NoError(t, err)
		require.Empty(t, jobs)
		require.Equal(t, 1, calls)
		require.Equal(t, []string{"resume:Vue.js"}, skills)
	})

	t.Run("zero skills does not call the API", func(t *testing.T) {
		t.Parallel()
		var calls int
		board := newStubBoard(t, func(http.ResponseWriter, *http.Request) {
			calls++
		})
		jobs, err := board.FetchUnion(context.Background(), []string{"python", "go", " Vue "})
		require.NoError(t, err)
		require.Empty(t, jobs)
		require.Zero(t, calls)
	})
}

func TestFetchUnion_tenantUnavailable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		get  int
		post int
	}{
		{name: "GET 404", get: http.StatusNotFound, post: http.StatusOK},
		{name: "POST 404", get: http.StatusOK, post: http.StatusNotFound},
		{name: "GET 410", get: http.StatusGone, post: http.StatusOK},
		{name: "POST 410", get: http.StatusOK, post: http.StatusGone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var posts int
			board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/jobs":
					w.WriteHeader(tc.get)
					if tc.get == http.StatusOK {
						_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
					}
				case "/api-boards/search-jobs":
					posts++
					w.WriteHeader(tc.post)
				default:
					http.Error(w, "unexpected", http.StatusInternalServerError)
				}
			})
			_, err := board.FetchUnion(context.Background(), []string{"vue"})
			require.Error(t, err)
			require.ErrorIs(t, err, collectors.ErrTenantUnavailable)
			if tc.get != http.StatusOK {
				require.Zero(t, posts)
			}
		})
	}
}

func TestFetchUnion_errors(t *testing.T) {
	t.Parallel()

	t.Run("429", func(t *testing.T) {
		t.Parallel()
		board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs" {
				_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
				return
			}
			w.WriteHeader(http.StatusTooManyRequests)
		})
		_, err := board.FetchUnion(context.Background(), []string{"vue"})
		require.Error(t, err)
		require.False(t, errors.Is(err, collectors.ErrTenantUnavailable))
		require.Contains(t, err.Error(), "429")
	})

	t.Run("missing csrf", func(t *testing.T) {
		t.Parallel()
		var posts int
		board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api-boards/search-jobs" {
				posts++
			}
			_, _ = io.WriteString(w, `<html>no token</html>`)
		})
		_, err := board.FetchUnion(context.Background(), []string{"react"})
		require.Error(t, err)
		require.False(t, errors.Is(err, collectors.ErrTenantUnavailable))
		require.Contains(t, err.Error(), "csrfToken")
		require.Zero(t, posts)
	})

	t.Run("no jobs array", func(t *testing.T) {
		t.Parallel()
		board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs" {
				_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
				return
			}
			_, _ = io.WriteString(w, `{"meta":{"sequence":"x"}}`)
		})
		_, err := board.FetchUnion(context.Background(), []string{"node"})
		require.Error(t, err)
		require.False(t, errors.Is(err, collectors.ErrTenantUnavailable))
		require.Contains(t, err.Error(), "jobs")
	})

	t.Run("empty jobs stops", func(t *testing.T) {
		t.Parallel()
		var posts int
		board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs" {
				_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
				return
			}
			posts++
			_, _ = io.WriteString(w, `{"jobs":[]}`)
		})
		jobs, err := board.FetchUnion(context.Background(), []string{"golang"})
		require.NoError(t, err)
		require.Empty(t, jobs)
		require.Equal(t, 1, posts)
	})
}

func TestFetchUnion_paginationStops(t *testing.T) {
	t.Parallel()

	t.Run("repeated job ids are not appended", func(t *testing.T) {
		t.Parallel()
		var posts int
		board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs" {
				_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
				return
			}
			posts++
			_, _ = io.WriteString(w, `{"meta":{"size":50,"sequence":"s-`+strconv.Itoa(posts)+`"},"jobs":[`+jobJSON("job-1", "Role")+`]}`)
		})
		jobs, err := board.FetchUnion(context.Background(), []string{"vue"})
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		require.Equal(t, 2, posts)
	})

	t.Run("missing sequence", func(t *testing.T) {
		t.Parallel()
		var posts int
		board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs" {
				_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
				return
			}
			posts++
			_, _ = io.WriteString(w, `{"meta":{"size":50},"jobs":[`+jobJSON("job-1", "Role")+`]}`)
		})
		jobs, err := board.FetchUnion(context.Background(), []string{"vue"})
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		require.Equal(t, 1, posts)
	})

	t.Run("max pages", func(t *testing.T) {
		t.Parallel()
		var posts int
		board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs" {
				_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
				return
			}
			posts++
			_, _ = io.WriteString(w, `{"meta":{"size":50,"sequence":"s-`+strconv.Itoa(posts)+`"},"jobs":[`+jobJSON("job-"+strconv.Itoa(posts), "Role")+`]}`)
		})
		board.MaxPages = 1
		jobs, err := board.FetchUnion(context.Background(), []string{"vue"})
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		require.Equal(t, 1, posts)
	})

	t.Run("zero max pages means 40", func(t *testing.T) {
		t.Parallel()
		var posts int
		board := newStubBoard(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs" {
				_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
				return
			}
			posts++
			_, _ = io.WriteString(w, `{"meta":{"size":50,"sequence":"s-`+strconv.Itoa(posts)+`"},"jobs":[`+jobJSON("job-"+strconv.Itoa(posts), "Role")+`]}`)
		})
		board.MaxPages = 0
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		jobs, err := board.FetchUnion(ctx, []string{"vue"})
		require.NoError(t, err)
		require.Len(t, jobs, defaultMaxPages)
		require.Equal(t, defaultMaxPages, posts)
	})
}

func TestFetch_doesNotDial(t *testing.T) {
	t.Parallel()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	t.Cleanup(srv.Close)

	board := &Board{
		HTTPClient: srv.Client(),
		Tenant:     Tenant{SourceID: "consider_gv", Host: srv.URL, BoardID: "gv"},
		MaxPages:   1,
	}
	jobs, err := board.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, jobs)
	require.Zero(t, calls)
}

func TestJobFromWire(t *testing.T) {
	t.Parallel()
	cr := testCountriesResolver(t)

	t.Run("apply fallback and scheme", func(t *testing.T) {
		t.Parallel()
		job, ok := jobFromWire("consider_gv", cr, jobWire{
			Title:         "Role",
			CompanyName:   "Acme",
			CompanyDomain: "https://acme.example/about",
			URL:           "https://jobs.example/1?gh_jid=9",
			Locations:     []string{"Berlin"},
			Hybrid:        true,
		})
		require.True(t, ok)
		require.Equal(t, "https://jobs.example/1?gh_jid=9", job.URL)
		require.Equal(t, "https://jobs.example/1?gh_jid=9", job.ApplyURL)
		require.Equal(t, "https://acme.example/about", job.CompanyWebsite)
		require.Equal(t, schema.LocationHybrid, job.Location.Type)
	})

	t.Run("office and one salary bound", func(t *testing.T) {
		t.Parallel()
		min := 100000.0
		job, ok := jobFromWire("consider_gv", cr, jobWire{
			Title:       "Role",
			CompanyName: "Acme",
			ApplyURL:    "https://jobs.example/apply",
			Locations:   []string{"Paris", " "},
			MinValue:    salaryAmount{n: &min},
			Currency:    "EUR",
		})
		require.True(t, ok)
		require.Equal(t, schema.LocationOffice, job.Location.Type)
		require.Equal(t, "Paris", job.Location.Raw)
		require.Equal(t, "100000- EUR", job.SalaryRaw)
		require.Empty(t, job.CompanyWebsite)
		require.Equal(t, "https://jobs.example/apply", job.ApplyURL)
		require.Empty(t, job.URL)
	})

	t.Run("drops incomplete hits", func(t *testing.T) {
		t.Parallel()
		_, ok := jobFromWire("consider_gv", cr, jobWire{Title: "", CompanyName: "Acme", URL: "https://jobs.example/1"})
		require.False(t, ok)
		_, ok = jobFromWire("consider_gv", cr, jobWire{Title: "Role", CompanyName: " ", URL: "https://jobs.example/1"})
		require.False(t, ok)
		_, ok = jobFromWire("consider_gv", cr, jobWire{Title: "Role", CompanyName: "Acme"})
		require.False(t, ok)
	})

	t.Run("unparseable time and no salary", func(t *testing.T) {
		t.Parallel()
		job, ok := jobFromWire("consider_gv", cr, jobWire{
			Title:       "Role",
			CompanyName: "Acme",
			URL:         "https://jobs.example/1",
			TimeStamp:   "yesterday",
		})
		require.True(t, ok)
		require.True(t, job.PostedAt.IsZero())
		require.Empty(t, job.SalaryRaw)
		require.Empty(t, job.Location.Type)
	})

	t.Run("golang once", func(t *testing.T) {
		t.Parallel()
		job, ok := jobFromWire("consider_gv", nil, jobWire{
			Title:       "Role",
			CompanyName: "Acme",
			URL:         "https://jobs.example/1",
			RequiredSkills: skillsField{
				{ID: "resume:Go", Label: "Go"},
				{ID: "resume:Go", Label: "Go"},
			},
			PreferredSkills: skillsField{
				{ID: "resume:Vue.js", Label: "Vue.js"},
			},
		})
		require.True(t, ok)
		require.Equal(t, "Go\ngolang\nVue.js", job.Description)
		require.Equal(t, []string{"Go", "Vue.js"}, job.Tags)
	})
}

func newStubBoard(t *testing.T, h http.HandlerFunc) *Board {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Board{
		HTTPClient: srv.Client(),
		Tenant: Tenant{
			SourceID: "consider_gv",
			Host:     srv.URL,
			BoardID:  "gv",
		},
		MaxPages: 5,
	}
}

func jobJSON(id, title string) string {
	b, err := json.Marshal(struct {
		JobID       string `json:"jobId"`
		Title       string `json:"title"`
		CompanyName string `json:"companyName"`
		URL         string `json:"url"`
	}{
		JobID:       id,
		Title:       title,
		CompanyName: "Acme",
		URL:         "https://jobs.example/" + id,
	})
	if err != nil {
		return ""
	}
	return string(b)
}
