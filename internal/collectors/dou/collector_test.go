package dou

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

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

func TestParseUkrainianPostedDisplay_withYear(t *testing.T) {
	anchor := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	got, ok := parseUkrainianPostedDisplay("8 квітня 2026", anchor)
	require.True(t, ok)
	want := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	require.True(t, got.Equal(want))
}

func TestParseUkrainianPostedDisplay_noYear(t *testing.T) {
	anchor := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	got, ok := parseUkrainianPostedDisplay("9 квітня", anchor)
	require.True(t, ok)
	want := time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC)
	require.True(t, got.Equal(want))
}

func TestResolvePostedAt_warnsOnUnknown(t *testing.T) {
	anchor := time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC)
	var warned []string
	got := resolvePostedAt(anchor, "nope", "also bad", func(s string) { warned = append(warned, s) })
	require.True(t, got.IsZero())
	require.Len(t, warned, 2)
}

func listingHTML(cardHREF, csrf string) string {
	return `<!doctype html><html><body>
<input type="hidden" name="csrfmiddlewaretoken" value="` + csrf + `">
<li class="l-vacancy">
  <div class="date">9 квітня</div>
  <div class="title">
    <a class="vt" href="` + cardHREF + `">Go Engineer</a>
    <strong><a class="company" href="/companies/acme/">Acme UA</a></strong>
  </div>
  <span class="cities">Україна</span>
</li>
</body></html>`
}

func detailHTML(title, body string) string {
	return detailHTMLWithPlace(title, body, "Київ, Україна")
}

func detailHTMLWithPlace(title, body, place string) string {
	if title == "" {
		title = "Go Engineer"
	}
	if body == "" {
		body = "Remote golang work."
	}
	if place == "" {
		place = "Київ, Україна"
	}
	return `<!doctype html><html><body>
<h1 class="g-h2">` + title + `</h1>
<div class="date">8 квітня 2026</div>
<div class="sh-info">
  <span class="place">` + place + `</span>
  <span class="salary">до $5000</span>
</div>
<div class="b-typo vacancy-section"><p>` + body + `</p></div>
<a class="badge" href="/vacancies/?category=backend">Backend</a>
</body></html>`
}

func TestFetch_remoteTrueWhenPlaceSaysVydaleno(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	base, err := url.Parse(srv.URL + "/")
	require.NoError(t, err)
	jobPath := "/companies/clario/vacancies/347763/"

	mux.HandleFunc("/vacancies/", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(listingHTML(jobPath, "csrf")))
	})
	mux.HandleFunc("/vacancies/xhr-load/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"html":"","last":true,"num":1}`))
	})
	mux.HandleFunc(jobPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		html := detailHTMLWithPlace("AI-native Engineer", "English description without the word remote.", "віддалено")
		_, _ = w.Write([]byte(html))
	})

	coll := &DOU{
		HTTPClient:        srv.Client(),
		Search:            "ai",
		ListingBase:       base,
		InterRequestDelay: 0,
		Countries:         testCountriesResolver(t),
	}
	jobs, err := coll.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotNil(t, jobs[0].Remote)
	require.True(t, *jobs[0].Remote, "віддалено in detail place should set remote")
}

func TestFetch_httptest_listingAndXHRAndDetail(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	base, err := url.Parse(srv.URL + "/")
	require.NoError(t, err)

	jobPath := "/companies/acme/vacancies/353313/"

	csrf := "fixturecsrf"
	mux.HandleFunc("/vacancies/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(listingHTML(jobPath, csrf)))
	})

	secondLi := `<li class="l-vacancy">
  <div class="date">7 квітня</div>
  <div class="title">
    <a class="vt" href="/companies/other/vacancies/353314/">Second Role</a>
    <strong><a class="company" href="/companies/other/">Other Co</a></strong>
  </div>
  <span class="cities">Україна</span>
</li>`
	xhrPayload, err := json.Marshal(map[string]any{
		"html": secondLi,
		"last": true,
		"num":  2,
	})
	require.NoError(t, err)

	var xhrPosts int
	mux.HandleFunc("/vacancies/xhr-load/", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		xhrPosts++
		require.NoError(t, r.ParseForm())
		require.Equal(t, csrf, r.FormValue("csrfmiddlewaretoken"))
		require.Equal(t, "1", r.FormValue("count"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(xhrPayload)
	})

	mux.HandleFunc(jobPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(detailHTML("Go Engineer", "Remote golang work.")))
	})
	mux.HandleFunc("/companies/other/vacancies/353314/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(detailHTML("Second Role", "Another remote job.")))
	})

	coll := &DOU{
		HTTPClient:        srv.Client(),
		Search:            "go",
		ListingBase:       base,
		InterRequestDelay: 0,
		Countries:         testCountriesResolver(t),
	}
	jobs, err := coll.Fetch(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, xhrPosts)
	require.Len(t, jobs, 2)

	j0 := jobs[0]
	require.Equal(t, SourceName, j0.Source)
	require.Equal(t, "Go Engineer", j0.Title)
	require.Equal(t, "Acme UA", j0.Company)
	require.Contains(t, j0.URL, "353313")
	require.Equal(t, "Remote golang work.", j0.Description)
	require.Equal(t, "UA", j0.CountryCode)
	require.Equal(t, "до $5000", j0.SalaryRaw)
	require.Equal(t, []string{"Backend"}, j0.Tags)
	wantPosted := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	require.True(t, j0.PostedAt.Equal(wantPosted))
	require.NotNil(t, j0.Remote)
	require.True(t, *j0.Remote)
	require.NotEmpty(t, j0.ID)

	require.Equal(t, "Second Role", jobs[1].Title)
	require.Equal(t, "Other Co", jobs[1].Company)
}

func TestFetch_maxJobsStopsBeforeSecondDetail(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	base, err := url.Parse(srv.URL + "/")
	require.NoError(t, err)
	u1 := "/companies/a/vacancies/1/"
	u2 := "/companies/b/vacancies/2/"

	list := `<!doctype html><html><body>
<input type="hidden" name="csrfmiddlewaretoken" value="x">
<li class="l-vacancy"><div class="date">1 січня</div><div class="title">
<a class="vt" href="` + u1 + `">A</a><strong><a class="company" href="#">Ca</a></strong></div></li>
<li class="l-vacancy"><div class="date">2 січня</div><div class="title">
<a class="vt" href="` + u2 + `">B</a><strong><a class="company" href="#">Cb</a></strong></div></li>
</body></html>`

	mux.HandleFunc("/vacancies/", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(list))
	})
	xhrPayload, err := json.Marshal(map[string]any{"html": "", "last": true, "num": 2})
	require.NoError(t, err)
	mux.HandleFunc("/vacancies/xhr-load/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(xhrPayload)
	})

	var detailGets int
	dh := func(w http.ResponseWriter, _ *http.Request) {
		detailGets++
		_, _ = w.Write([]byte(detailHTML("T", "x")))
	}
	mux.HandleFunc(u1, dh)
	mux.HandleFunc(u2, dh)

	coll := &DOU{
		HTTPClient:        srv.Client(),
		Search:            "q",
		ListingBase:       base,
		MaxJobs:           1,
		InterRequestDelay: 0,
		Countries:         testCountriesResolver(t),
	}
	jobs, err := coll.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, 1, detailGets)
}
