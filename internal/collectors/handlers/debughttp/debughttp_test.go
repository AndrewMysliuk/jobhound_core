package debughttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

type stubCollector struct {
	name string
	jobs []domain.Job
	err  error
}

func (s stubCollector) Name() string {
	if s.name != "" {
		return s.name
	}
	return "stub"
}

func (s stubCollector) Fetch(context.Context) ([]domain.Job, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.jobs, nil
}

func TestHealth(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(NewMux(stubCollector{name: "europe_remotely"}, stubCollector{name: "working_nomads"}, nil, nil))
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body %#v", body)
	}
}

func TestEuropeRemotely_ok(t *testing.T) {
	t.Parallel()
	er := stubCollector{
		name: "europe_remotely",
		jobs: []domain.Job{
			{ID: "a", Title: "T1", Source: "europe_remotely"},
		},
	}
	wn := stubCollector{name: "working_nomads"}
	srv := httptest.NewServer(NewMux(er, wn, nil, nil))
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/debug/collectors/europe_remotely", "application/json", strings.NewReader(`{"limit":0}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out runCollectorResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Collector != "europe_remotely" || out.Count != 1 || len(out.Jobs) != 1 {
		t.Fatalf("%+v", out)
	}
	if out.Jobs[0].Title != "T1" || out.Jobs[0].ID != "a" {
		t.Fatalf("job json: %+v", out.Jobs[0])
	}
}

func TestWorkingNomads_ok(t *testing.T) {
	t.Parallel()
	er := stubCollector{name: "europe_remotely"}
	wn := stubCollector{
		name: "working_nomads",
		jobs: []domain.Job{
			{ID: "b", Title: "T2", Source: "working_nomads"},
			{ID: "c", Title: "T3", Source: "working_nomads"},
		},
	}
	srv := httptest.NewServer(NewMux(er, wn, nil, nil))
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/debug/collectors/working_nomads", "application/json", strings.NewReader(`{"limit":0}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out runCollectorResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Collector != "working_nomads" || out.Count != 2 || len(out.Jobs) != 2 {
		t.Fatalf("%+v", out)
	}
}

func TestEuropeRemotely_fetchError(t *testing.T) {
	t.Parallel()
	er := stubCollector{name: "europe_remotely", err: errors.New("boom")}
	wn := stubCollector{name: "working_nomads"}
	srv := httptest.NewServer(NewMux(er, wn, nil, nil))
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/debug/collectors/europe_remotely", "application/json", strings.NewReader(`{"limit":0}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out runCollectorResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.OK || out.Error != "boom" {
		t.Fatalf("%+v", out)
	}
}

func TestEuropeRemotely_defaultLimitTruncates(t *testing.T) {
	t.Parallel()
	jobs := make([]domain.Job, 250)
	for i := range jobs {
		jobs[i] = domain.Job{ID: fmt.Sprintf("id-%d", i), Title: "T", Source: "europe_remotely"}
	}
	er := stubCollector{name: "europe_remotely", jobs: jobs}
	wn := stubCollector{name: "working_nomads"}
	srv := httptest.NewServer(NewMux(er, wn, nil, nil))
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/debug/collectors/europe_remotely", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out runCollectorResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Count != 200 || len(out.Jobs) != 200 || out.UpstreamFetched != 250 {
		t.Fatalf("got count=%d jobs=%d upstream=%d", out.Count, len(out.Jobs), out.UpstreamFetched)
	}
}

func TestInvalidLimitBody(t *testing.T) {
	t.Parallel()
	er := stubCollector{name: "europe_remotely"}
	wn := stubCollector{name: "working_nomads"}
	srv := httptest.NewServer(NewMux(er, wn, nil, nil))
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/debug/collectors/europe_remotely", "application/json", strings.NewReader(`{"limit":-1}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestEuropeRemotely_debugPassesSearchKeywordsToFeed(t *testing.T) {
	mux := http.NewServeMux()
	upstream := httptest.NewServer(mux)
	t.Cleanup(upstream.Close)

	listingURL := upstream.URL + "/job/one/"
	htmlFrag := `<div class="job-card"><h2 class="job-title"><a href="` + listingURL + `">T</a></h2><div class="company-name">Co</div><div class="meta-item meta-location">Germany</div><div class="job-time">Posted 1 day ago</div></div>`
	detailHTML := `<div class="page-header"><h1 class="page-title">T</h1><h3 class="page-subtitle"><ul class="job-listing-meta meta"><li class="job-company">Co</li><li class="date-posted">March 30, 2026</li></ul></h3></div><div class="job_listing-description"><p>x</p></div>`
	feedBytes, err := json.Marshal(map[string]any{"has_more": false, "html": htmlFrag})
	require.NoError(t, err)

	var gotKeywords string
	mux.HandleFunc("/ajax", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		gotKeywords = r.FormValue("search_keywords")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(feedBytes)
	})
	dh := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(detailHTML))
	}
	mux.HandleFunc("/job/one", dh)
	mux.HandleFunc("/job/one/", dh)

	siteBase, err := url.Parse(upstream.URL + "/")
	require.NoError(t, err)
	er := &europeremotely.EuropeRemotely{
		HTTPClient: upstream.Client(),
		FeedURL:    upstream.URL + "/ajax",
		FeedForm:   url.Values{"action": {"erj_ajax_search"}, "nonce": {"n"}, "website": {""}},
		SiteBase:   siteBase,
	}
	wn := stubCollector{name: "working_nomads"}
	dbg := httptest.NewServer(NewMux(er, wn, nil, er))
	t.Cleanup(dbg.Close)

	res, err := http.Post(dbg.URL+"/debug/collectors/europe_remotely", "application/json", strings.NewReader(`{"limit":1,"search_keywords":"vue"}`))
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "vue", gotKeywords)
	var out runCollectorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	require.True(t, out.OK)
	require.Equal(t, 1, out.Count)
}
