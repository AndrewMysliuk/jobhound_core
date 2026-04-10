package djinni

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestDjinni_fetch_listingAndDetail_httptest(t *testing.T) {
	t.Parallel()
	const jobPath = "/jobs/424242-fixture-role"
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimSuffix(r.URL.Path, "/")
		switch p {
		case jobPath:
			ld := map[string]any{
				"@context":        "https://schema.org",
				"@type":           "JobPosting",
				"identifier":      "424242",
				"url":             strings.TrimRight(upstream.URL, "/") + jobPath,
				"title":           "Senior Frontend Engineer",
				"description":     "<p>Build <b>React</b> apps. Remote-friendly.</p>",
				"datePosted":      "2026-03-28T10:00:00Z",
				"category":        "Fullstack",
				"jobLocationType": "TELECOMMUTE",
				"hiringOrganization": map[string]any{
					"@type": "Organization",
					"name":  "Acme Corp",
				},
				"applicantLocationRequirements": map[string]any{
					"@type":          "Country",
					"name":           "Ukraine",
					"addressCountry": "UA",
				},
				"baseSalary": map[string]any{
					"@type":    "MonetaryAmount",
					"currency": "USD",
					"value": map[string]any{
						"@type":    "QuantitativeValue",
						"minValue": 550,
						"maxValue": 650,
						"unitText": "MONTH",
					},
				},
			}
			b, err := json.Marshal(ld)
			require.NoError(t, err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><body><script type=\"application/ld+json\">\n" + string(b) + "\n</script></body></html>"))
		case "/jobs":
			listing := `<html><body><div>
<a href="` + jobPath + `/"><h2 class="job-item__position">Senior Frontend Engineer</h2></a>
<span class="small text-gray-800 opacity-75 font-weight-500">Acme Corp</span>
<div class="fw-medium"><span class="text-nowrap">Тільки віддалено</span><span class="location-text">Ukraine</span></div>
<div class="job-item__tags"><span class="badge">React</span></div>
<div class="col-auto"><div class="fs-5"><strong class="text-success">до $650</strong></div></div>
</div></body></html>`
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(listing))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	base, err := url.Parse(upstream.URL + "/")
	require.NoError(t, err)
	c := &Djinni{
		HTTPClient:        upstream.Client(),
		SiteBase:          base,
		AllKeywords:       "fixture",
		MaxJobs:           5,
		InterRequestDelay: 0,
		StartPage:         1,
	}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	j := jobs[0]
	require.Equal(t, SourceName, j.Source)
	require.Equal(t, "Senior Frontend Engineer", j.Title)
	require.Equal(t, "Acme Corp", j.Company)
	require.Contains(t, j.SalaryRaw, "550")
	require.Contains(t, j.SalaryRaw, "650")
	require.Contains(t, j.SalaryRaw, "USD")
	require.True(t, j.PostedAt.Equal(time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC)))
	require.NotNil(t, j.Remote)
	require.True(t, *j.Remote)
	require.Equal(t, "UA", j.CountryCode)
	require.Contains(t, j.Tags, "Fullstack")
	require.Contains(t, j.Tags, "React")
	require.Contains(t, j.Description, "React")
	require.NotContains(t, j.Description, "<p>")
	id, err := domainutils.StableJobID(SourceName, j.URL)
	require.NoError(t, err)
	require.Equal(t, id, j.ID)
}

func TestDjinni_fetch_badDatePosted_softFailAndWarn(t *testing.T) {
	t.Parallel()
	const jobPath = "/jobs/424243-other-role"
	var warned string
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimSuffix(r.URL.Path, "/")
		switch p {
		case jobPath:
			ld := map[string]any{
				"@context":    "https://schema.org",
				"@type":       "JobPosting",
				"url":         strings.TrimRight(upstream.URL, "/") + jobPath,
				"title":       "Other Role",
				"description": "Work remotely from anywhere.",
				"datePosted":  "not-an-iso-date",
				"hiringOrganization": map[string]any{
					"@type": "Organization",
					"name":  "Co",
				},
			}
			b, err := json.Marshal(ld)
			require.NoError(t, err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><body><script type=\"application/ld+json\">" + string(b) + "</script></body></html>"))
		case "/jobs":
			listing := `<html><body><a href="` + jobPath + `/"><h2 class="job-item__position">Other Role</h2></a>
<span class="small text-gray-800 opacity-75 font-weight-500">Co</span></body></html>`
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(listing))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	base, err := url.Parse(upstream.URL + "/")
	require.NoError(t, err)
	c := &Djinni{
		HTTPClient:        upstream.Client(),
		SiteBase:          base,
		AllKeywords:       "x",
		MaxJobs:           3,
		InterRequestDelay: 0,
		OnDatePostedWarn: func(raw string) {
			warned = raw
		},
	}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.True(t, jobs[0].PostedAt.IsZero())
	require.Equal(t, "not-an-iso-date", warned)
	require.NotNil(t, jobs[0].Remote)
	require.True(t, *jobs[0].Remote)
}

func TestDjinni_datePosted_fractionalSecondsNoTimezone(t *testing.T) {
	t.Parallel()
	const jobPath = "/jobs/818071-live-shape"
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimSuffix(r.URL.Path, "/")
		switch p {
		case jobPath:
			ld := map[string]any{
				"@context":   "https://schema.org/",
				"@type":      "JobPosting",
				"url":        strings.TrimRight(upstream.URL, "/") + jobPath,
				"title":      "Senior Back-End Developer",
				"datePosted": "2026-04-10T12:07:48.216890",
				"hiringOrganization": map[string]any{
					"@type": "Organization",
					"name":  "TalentMinted",
				},
				"description": "Remote within Canada.",
			}
			b, err := json.Marshal(ld)
			require.NoError(t, err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><body><script type=\"application/ld+json\">" + string(b) + "</script></body></html>"))
		case "/jobs":
			listing := `<html><body><a href="` + jobPath + `/"><h2 class="job-item__position">Senior Back-End Developer</h2></a>
<span class="small text-gray-800 opacity-75 font-weight-500">Co</span></body></html>`
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(listing))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	base, err := url.Parse(upstream.URL + "/")
	require.NoError(t, err)
	c := &Djinni{
		HTTPClient:        upstream.Client(),
		SiteBase:          base,
		AllKeywords:       "q",
		MaxJobs:           2,
		InterRequestDelay: 0,
	}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	want := time.Date(2026, 4, 10, 12, 7, 48, 216890000, time.UTC)
	require.True(t, jobs[0].PostedAt.Equal(want), "got %v want %v", jobs[0].PostedAt, want)
}

func TestDjinni_remote_vidilenaRobota_inListingMeta(t *testing.T) {
	t.Parallel()
	const jobPath = "/jobs/999-hybrid-line"
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimSuffix(r.URL.Path, "/")
		switch p {
		case jobPath:
			ld := map[string]any{
				"@type":       "JobPosting",
				"url":         strings.TrimRight(upstream.URL, "/") + jobPath,
				"title":       "Engineer",
				"description": "Office first.",
				"hiringOrganization": map[string]any{
					"name": "Co",
				},
			}
			b, err := json.Marshal(ld)
			require.NoError(t, err)
			_, _ = w.Write([]byte("<html><body><script type=\"application/ld+json\">" + string(b) + "</script></body></html>"))
		case "/jobs":
			listing := `<html><body><div>
<a href="` + jobPath + `/"><h2 class="job-item__position">Engineer</h2></a>
<span class="small text-gray-800 opacity-75 font-weight-500">Co</span>
<div class="fw-medium"><span class="d-block font-weight-600">Офіс, Віддалена робота, Гібридний формат роботи</span></div>
</div></body></html>`
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(listing))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	base, err := url.Parse(upstream.URL + "/")
	require.NoError(t, err)
	c := &Djinni{
		HTTPClient:        upstream.Client(),
		SiteBase:          base,
		AllKeywords:       "q",
		MaxJobs:           2,
		InterRequestDelay: 0,
	}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotNil(t, jobs[0].Remote)
	require.True(t, *jobs[0].Remote, "віддалена робота in listing meta should set Remote true")
}

func TestDjinni_FetchWithSlotSearch_overridesKeywords(t *testing.T) {
	t.Parallel()
	const jobPath = "/jobs/1-a"
	calls := 0
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		p := strings.TrimSuffix(r.URL.Path, "/")
		if p == "/jobs" {
			if got := r.URL.Query().Get("all_keywords"); got != "golang" {
				http.Error(w, "wrong keywords", http.StatusBadRequest)
				return
			}
			listing := `<html><body><a href="` + jobPath + `/"><h2 class="job-item__position">X</h2></a>
<span class="small text-gray-800 opacity-75 font-weight-500">Y</span></body></html>`
			_, _ = w.Write([]byte(listing))
			return
		}
		if p == jobPath {
			ld := map[string]any{
				"@type":       "JobPosting",
				"url":         strings.TrimRight(upstream.URL, "/") + jobPath,
				"title":       "X",
				"description": "desc",
				"hiringOrganization": map[string]any{
					"name": "Y",
				},
			}
			b, _ := json.Marshal(ld)
			_, _ = w.Write([]byte("<html><script type=\"application/ld+json\">" + string(b) + "</script></html>"))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(upstream.Close)
	base, _ := url.Parse(upstream.URL + "/")
	c := &Djinni{
		HTTPClient:        upstream.Client(),
		SiteBase:          base,
		AllKeywords:       "wrong",
		MaxJobs:           2,
		InterRequestDelay: 0,
	}
	_, err := c.FetchWithSlotSearch(context.Background(), "golang")
	require.NoError(t, err)
	require.GreaterOrEqual(t, calls, 2)
}
