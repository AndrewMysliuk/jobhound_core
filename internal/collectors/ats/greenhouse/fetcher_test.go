package greenhouse

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	companyschema "github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

const fallbackCompany = "Fallback Co"

func TestFetchBoard_outcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		bodyFile string
		etagIn   string
		wantOut  companyschema.PollOutcome
		wantJobs int
		wantETag string
		checkReq func(t *testing.T, r *http.Request)
	}{
		{
			name:     "complete",
			status:   http.StatusOK,
			bodyFile: "jobs_ok.json",
			wantOut:  companyschema.PollOutcomeComplete,
			wantJobs: 1,
			wantETag: `"etag-ok"`,
			checkReq: func(t *testing.T, r *http.Request) {
				t.Helper()
				if !strings.Contains(r.URL.RawQuery, "content=true") {
					t.Fatalf("query %q missing content=true", r.URL.RawQuery)
				}
				if r.Header.Get("If-None-Match") != "" {
					t.Fatalf("unexpected If-None-Match")
				}
			},
		},
		{
			name:     "empty",
			status:   http.StatusOK,
			bodyFile: "jobs_empty.json",
			wantOut:  companyschema.PollOutcomeEmpty,
			wantJobs: 0,
		},
		{
			name:     "dead_slug_404",
			status:   http.StatusNotFound,
			bodyFile: "jobs_404.json",
			wantOut:  companyschema.PollOutcomeDeadSlug,
			wantJobs: 0,
		},
		{
			name:     "not_modified_304",
			status:   http.StatusNotModified,
			etagIn:   `"prev"`,
			wantOut:  companyschema.PollOutcomeNotModified,
			wantJobs: 0,
			wantETag: `"prev"`,
			checkReq: func(t *testing.T, r *http.Request) {
				t.Helper()
				if got := r.Header.Get("If-None-Match"); got != `"prev"` {
					t.Fatalf("If-None-Match=%q", got)
				}
			},
		},
		{
			name:     "partial_meta_mismatch",
			status:   http.StatusOK,
			bodyFile: "jobs_partial.json",
			wantOut:  companyschema.PollOutcomePartial,
			wantJobs: 1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var sawContentTrue bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.checkReq != nil {
					tc.checkReq(t, r)
				}
				if strings.Contains(r.URL.RawQuery, "content=true") {
					sawContentTrue = true
				}
				if tc.wantETag != "" && tc.status == http.StatusOK {
					w.Header().Set("ETag", tc.wantETag)
				}
				w.WriteHeader(tc.status)
				if tc.bodyFile != "" {
					b, err := os.ReadFile(filepath.Join("testdata", tc.bodyFile))
					if err != nil {
						t.Fatalf("read fixture: %v", err)
					}
					_, _ = w.Write(b)
				}
			}))
			t.Cleanup(srv.Close)

			c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
			res, err := c.FetchBoard(context.Background(), "storyblok", fallbackCompany, tc.etagIn)
			if err != nil {
				t.Fatalf("FetchBoard: %v", err)
			}
			if res.Outcome != tc.wantOut {
				t.Fatalf("outcome=%s want %s", res.Outcome, tc.wantOut)
			}
			if len(res.Jobs) != tc.wantJobs {
				t.Fatalf("jobs=%d want %d", len(res.Jobs), tc.wantJobs)
			}
			if tc.wantETag != "" && res.ETag != tc.wantETag {
				t.Fatalf("etag=%q want %q", res.ETag, tc.wantETag)
			}
			if tc.status == http.StatusOK || tc.status == http.StatusNotModified {
				if !sawContentTrue && tc.status == http.StatusOK {
					t.Fatal("request missing content=true")
				}
			}
			if tc.wantJobs > 0 {
				j := res.Jobs[0]
				if j.ID == "" || j.CompanyKey == "" {
					t.Fatalf("AssignStableID missing: id=%q key=%q", j.ID, j.CompanyKey)
				}
				if j.Company != "Storyblok" {
					t.Fatalf("company=%q", j.Company)
				}
				if j.Description == "" || strings.Contains(j.Description, "<p>") {
					t.Fatalf("description not plain: %q", j.Description)
				}
				if j.ApplyURL == "" {
					t.Fatal("missing ApplyURL")
				}
				if j.Location.Type == "" {
					t.Fatal("expected location type from Remote office")
				}
				wantID := domainutils.StableJobID(domainutils.CompanyKey("Storyblok"), j.Title)
				if j.ID != wantID {
					t.Fatalf("id=%q want %q", j.ID, wantID)
				}
			}
		})
	}
}

func TestFetchBoard_contentTrueAlwaysPresent(t *testing.T) {
	t.Parallel()
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		w.Header().Set("ETag", `"x"`)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"jobs":[],"meta":{"total":0}}`)
	}))
	t.Cleanup(srv.Close)

	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	if _, err := c.FetchBoard(context.Background(), "acme", fallbackCompany, ""); err != nil {
		t.Fatal(err)
	}
	if rawQuery != "content=true" && !strings.Contains(rawQuery, "content=true") {
		t.Fatalf("RawQuery=%q", rawQuery)
	}
}

func TestFetchBoard_slugVerbatim(t *testing.T) {
	t.Parallel()
	for _, slug := range []string{"primer.io", "Coder", "541", "ecosia.org"} {
		slug := slug
		t.Run(slug, func(t *testing.T) {
			t.Parallel()
			var path string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path = r.URL.Path
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"status":404}`)
			}))
			t.Cleanup(srv.Close)
			c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
			res, err := c.FetchBoard(context.Background(), slug, fallbackCompany, "")
			if err != nil {
				t.Fatal(err)
			}
			if res.Outcome != companyschema.PollOutcomeDeadSlug {
				t.Fatalf("outcome=%s", res.Outcome)
			}
			want := "/v1/boards/" + slug + "/jobs"
			if path != want {
				t.Fatalf("path=%q want %q", path, want)
			}
		})
	}
}

func TestFetchBoard_404DoesNotDecodeAsFormatError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `Not Found`) // plain text, like a bad decode target
	}))
	t.Cleanup(srv.Close)
	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	res, err := c.FetchBoard(context.Background(), "nope", fallbackCompany, "")
	if err != nil {
		t.Fatalf("404 must not be an error, got %v", err)
	}
	if res.Outcome != companyschema.PollOutcomeDeadSlug {
		t.Fatalf("outcome=%s", res.Outcome)
	}
}

func TestFetchBoard_companyNameStampsWhenBoardOmits(t *testing.T) {
	t.Parallel()
	body := `{
  "jobs": [{
    "id": 1,
    "title": "Engineer",
    "absolute_url": "https://boards.greenhouse.io/acme/jobs/1",
    "company_name": "",
    "content": "<p>Go role.</p>",
    "location": {"name": "Remote"},
    "first_published": "2026-10-07T05:40:48-04:00",
    "updated_at": "2026-10-20T10:19:32-04:00",
    "departments": [],
    "offices": [{"id": 1, "name": "Remote", "location": null}]
  }],
  "meta": {"total": 1}
}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	const company = "Acme Corp"
	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	res, err := c.FetchBoard(context.Background(), "acme", company, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 1 {
		t.Fatalf("jobs=%d", len(res.Jobs))
	}
	j := res.Jobs[0]
	if j.Company != company {
		t.Fatalf("company=%q want fallback %q", j.Company, company)
	}
	wantKey := domainutils.CompanyKey(company)
	if j.CompanyKey != wantKey || wantKey == "" {
		t.Fatalf("companyKey=%q want %q", j.CompanyKey, wantKey)
	}
	if strings.Contains(j.ID, "http") || strings.Contains(j.ID, j.URL) {
		t.Fatalf("id must not contain listing URL: %q", j.ID)
	}
}

func TestFetchBoard_boardCompanyNameWinsOverArg(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("testdata", "jobs_ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)

	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	res, err := c.FetchBoard(context.Background(), "storyblok", "Ignored Fallback", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 1 {
		t.Fatalf("jobs=%d", len(res.Jobs))
	}
	if res.Jobs[0].Company != "Storyblok" {
		t.Fatalf("company=%q want board company_name", res.Jobs[0].Company)
	}
}
