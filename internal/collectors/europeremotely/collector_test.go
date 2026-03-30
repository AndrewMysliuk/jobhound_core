package europeremotely

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

const feedFixtureJSON = `{
  "has_more": false,
  "html": "<div class=\"job-card\"><h2 class=\"job-title\"><a href=\"https://euremotejobs.com/job/example-slug/\">Senior Go Engineer</a></h2><div class=\"company-name\">Acme EU</div><div class=\"meta-item meta-location\">Germany, Remote</div><div class=\"meta-item meta-type\">Full Time</div><div class=\"job-time\">Posted 2 days ago</div></div>"
}`

const detailFixtureHTML = `<div class="page-header">
  <h1 class="page-title">Senior Go Engineer</h1>
  <h3 class="page-subtitle">
    <ul class="job-listing-meta meta">
      <li class="job-type">Full Time</li>
      <li class="location"><a class="google_map_link" href="https://www.google.com/maps?q=Berlin">Berlin, Germany</a></li>
      <li class="date-posted">March 28, 2026</li>
      <li class="job-company"><a href="https://euremotejobs.com/company/acme/">Acme EU</a></li>
      <li class="wpjmef-field-salary">Salary: 90,000–110,000 EUR/year</li>
    </ul>
  </h3>
</div>
<div class="job_listing-description"><p>Build distributed systems.</p></div>
<p class="job_tags">Tagged as: golang, backend, remote</p>
<a class="application_button_link" href="https://ats.example.com/apply/1">Apply for job</a>`

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

func TestDecodeFeedEnvelope_wordPressWrapper(t *testing.T) {
	inner := `<div class="job-card"><h2 class="job-title"><a href="/job/x/">T</a></h2></div>`
	htmlJSON, err := json.Marshal(inner)
	require.NoError(t, err)
	raw := `{"success":true,"data":{"has_more":false,"html":` + string(htmlJSON) + `}}`
	hasMore, html, err := decodeFeedEnvelope([]byte(raw))
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Contains(t, html, "job-card")
}

func TestParseListingCards_wrappedInAnchor(t *testing.T) {
	frag := `<a href="https://euremotejobs.com/job/example-slug/" class="job-card-link"><div class="job-card"><div class="job-details"><h2 class="job-title">Senior Go Engineer</h2><div class="company-name">Acme EU</div><div class="job-meta"><div class="meta-item meta-location">Germany</div></div></div><div class="job-time">Posted 2 days ago</div></div></a>`
	base, err := url.Parse("https://euremotejobs.com/")
	require.NoError(t, err)
	cards, err := parseListingCards(frag, base)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Equal(t, "https://euremotejobs.com/job/example-slug/", cards[0].jobPageURL)
	require.Equal(t, "Senior Go Engineer", cards[0].title)
}

func TestParseListingFeedJSON(t *testing.T) {
	var env feedEnvelope
	require.NoError(t, json.Unmarshal([]byte(feedFixtureJSON), &env))
	base, err := url.Parse("https://euremotejobs.com/")
	require.NoError(t, err)
	cards, err := parseListingCards(env.HTML, base)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Equal(t, "Senior Go Engineer", cards[0].title)
	require.Equal(t, "Acme EU", cards[0].company)
	canon, err := utils.CanonicalListingURL(cards[0].jobPageURL)
	require.NoError(t, err)
	require.Equal(t, "https://euremotejobs.com/job/example-slug", canon)
}

func TestParseDetailHTML(t *testing.T) {
	base, err := url.Parse("https://euremotejobs.com/")
	require.NoError(t, err)
	d, err := parseJobDetailHTML(detailFixtureHTML, base)
	require.NoError(t, err)
	require.Equal(t, "Senior Go Engineer", d.title)
	require.Equal(t, "https://ats.example.com/apply/1", d.applyURL)
	require.Equal(t, "Build distributed systems.", d.description)
	require.Equal(t, []string{"golang", "backend", "remote"}, d.tags)
}

func TestFetch_httptest(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	listingURL := srv.URL + "/job/example-slug/"
	htmlFrag := `<div class="job-card"><h2 class="job-title"><a href="` + listingURL + `">Senior Go Engineer</a></h2><div class="company-name">Acme EU</div><div class="meta-item meta-location">Germany, Remote</div><div class="meta-item meta-type">Full Time</div><div class="job-time">Posted 2 days ago</div></div>`
	feedBytes, err := json.Marshal(map[string]any{
		"has_more": false,
		"html":     htmlFrag,
	})
	require.NoError(t, err)

	mux.HandleFunc("/ajax", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(feedBytes)
	})
	detailHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(detailFixtureHTML))
	}
	mux.HandleFunc("/job/example-slug", detailHandler)
	mux.HandleFunc("/job/example-slug/", detailHandler)

	siteBase, err := url.Parse(srv.URL + "/")
	require.NoError(t, err)
	coll := &EuropeRemotely{
		HTTPClient: srv.Client(),
		FeedURL:    srv.URL + "/ajax",
		FeedForm:   url.Values{"action": {"test"}},
		SiteBase:   siteBase,
		Countries:  testCountriesResolver(t),
	}
	jobs, err := coll.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	j := jobs[0]
	require.Equal(t, SourceName, j.Source)
	require.Equal(t, "Senior Go Engineer", j.Title)
	require.Equal(t, "Acme EU", j.Company)
	require.Contains(t, j.URL, "/job/example-slug")
	require.Equal(t, "https://ats.example.com/apply/1", j.ApplyURL)
	require.Equal(t, "Build distributed systems.", j.Description)
	wantPosted, err := time.Parse("January 2, 2006", "March 28, 2026")
	require.NoError(t, err)
	require.True(t, j.PostedAt.Equal(wantPosted.UTC()))
	require.Equal(t, "DE", j.CountryCode)
	require.NotNil(t, j.Remote)
	require.True(t, *j.Remote)
	require.Contains(t, j.SalaryRaw, "90,000")
	require.Equal(t, []string{"golang", "backend", "remote"}, j.Tags)
	require.NotNil(t, j.Position)
	require.Equal(t, "backend", *j.Position)
	require.NotEmpty(t, j.ID)
}

func TestFetch_maxJobsStopsEarly(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	u1 := srv.URL + "/job/one/"
	u2 := srv.URL + "/job/two/"
	htmlFrag := `<div class="job-card"><h2 class="job-title"><a href="` + u1 + `">First</a></h2><div class="company-name">A</div><div class="meta-item meta-location">DE</div><div class="job-time">Posted 1 day ago</div></div>` +
		`<div class="job-card"><h2 class="job-title"><a href="` + u2 + `">Second</a></h2><div class="company-name">B</div><div class="meta-item meta-location">DE</div><div class="job-time">Posted 2 days ago</div></div>`
	feedBytes, err := json.Marshal(map[string]any{"has_more": false, "html": htmlFrag})
	require.NoError(t, err)

	var detailGets int
	mux.HandleFunc("/ajax", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(feedBytes)
	})
	detailHandler := func(w http.ResponseWriter, _ *http.Request) {
		detailGets++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(detailFixtureHTML))
	}
	mux.HandleFunc("/job/one", detailHandler)
	mux.HandleFunc("/job/one/", detailHandler)
	mux.HandleFunc("/job/two", detailHandler)
	mux.HandleFunc("/job/two/", detailHandler)

	siteBase, err := url.Parse(srv.URL + "/")
	require.NoError(t, err)
	coll := &EuropeRemotely{
		HTTPClient: srv.Client(),
		FeedURL:    srv.URL + "/ajax",
		FeedForm:   url.Values{"action": {"test"}},
		SiteBase:   siteBase,
		Countries:  testCountriesResolver(t),
		MaxJobs:    1,
	}
	jobs, err := coll.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, 1, detailGets, "second job detail should not be fetched when MaxJobs=1")
}

func TestResolvePostedAt_relativeClock(t *testing.T) {
	anchor := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	var warned []string
	got := resolvePostedAt(anchor, "Posted 2 days ago", "", func(raw string) {
		warned = append(warned, raw)
	})
	want := anchor.Add(-48 * time.Hour)
	require.True(t, got.Equal(want.UTC()))
	require.Empty(t, warned)
}
