package ashby

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	companyschema "github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	domainschema "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetchBoard_outcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		bodyFile string
		bodyRaw  string
		etagIn   string
		maxJobs  int
		wantOut  companyschema.PollOutcome
		wantJobs int
		wantETag string
		company  string
	}{
		{
			name:     "complete",
			status:   http.StatusOK,
			bodyFile: "jobs_ok.json",
			wantOut:  companyschema.PollOutcomeComplete,
			wantJobs: 1,
			wantETag: `"etag-ok"`,
			company:  "n8n",
		},
		{
			name:     "empty",
			status:   http.StatusOK,
			bodyFile: "jobs_empty.json",
			wantOut:  companyschema.PollOutcomeEmpty,
			company:  "n8n",
		},
		{
			name:     "dead_slug_plain_404",
			status:   http.StatusNotFound,
			bodyFile: "jobs_404.txt",
			wantOut:  companyschema.PollOutcomeDeadSlug,
			company:  "n8n",
		},
		{
			name:     "not_modified",
			status:   http.StatusNotModified,
			etagIn:   `"prev"`,
			wantOut:  companyschema.PollOutcomeNotModified,
			wantETag: `"prev"`,
			company:  "n8n",
		},
		{
			name:     "partial_max_jobs",
			status:   http.StatusOK,
			bodyFile: "jobs_two.json",
			maxJobs:  1,
			wantOut:  companyschema.PollOutcomePartial,
			wantJobs: 1,
			company:  "n8n",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotINM string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotINM = r.Header.Get("If-None-Match")
				if tc.wantETag != "" && tc.status == http.StatusOK {
					w.Header().Set("ETag", tc.wantETag)
				}
				w.WriteHeader(tc.status)
				switch {
				case tc.bodyFile != "":
					b, err := os.ReadFile(filepath.Join("testdata", tc.bodyFile))
					if err != nil {
						t.Fatalf("fixture: %v", err)
					}
					_, _ = w.Write(b)
				case tc.bodyRaw != "":
					_, _ = io.WriteString(w, tc.bodyRaw)
				}
			}))
			t.Cleanup(srv.Close)

			c := &Client{
				HTTPClient: srv.Client(),
				BaseURL:    srv.URL,
				MaxJobs:    tc.maxJobs,
			}
			res, err := c.FetchBoard(context.Background(), "Coder", tc.company, tc.etagIn)
			if err != nil {
				t.Fatalf("FetchBoard: %v", err)
			}
			if res.Outcome != tc.wantOut {
				t.Fatalf("outcome=%s want %s", res.Outcome, tc.wantOut)
			}
			if len(res.Jobs) != tc.wantJobs {
				t.Fatalf("jobs=%d want %d", len(res.Jobs), tc.wantJobs)
			}
			if tc.etagIn != "" && gotINM != tc.etagIn {
				t.Fatalf("If-None-Match=%q want %q", gotINM, tc.etagIn)
			}
			if tc.wantETag != "" && res.ETag != tc.wantETag {
				t.Fatalf("etag=%q want %q", res.ETag, tc.wantETag)
			}
			if tc.wantJobs > 0 {
				assertMappedJob(t, res.Jobs[0], tc.company)
			}
		})
	}
}

func assertMappedJob(t *testing.T, j domainschema.Job, company string) {
	t.Helper()
	if j.Source != SourceName {
		t.Fatalf("source=%q", j.Source)
	}
	if j.Company != company {
		t.Fatalf("company=%q", j.Company)
	}
	if j.Description == "" {
		t.Fatal("empty description")
	}
	if j.ApplyURL == "" {
		t.Fatal("empty apply url")
	}
	if j.Location.Type != domainschema.LocationRemote {
		t.Fatalf("location type=%q want remote", j.Location.Type)
	}
	if !j.PostedAt.IsZero() && j.PostedAt.Year() < 2020 {
		t.Fatalf("postedAt=%v", j.PostedAt)
	}
	want := domainutils.StableJobID(domainutils.CompanyKey(company), j.Title)
	if j.ID != want {
		t.Fatalf("id=%q want %q", j.ID, want)
	}
}

func TestFetchBoard_companyNameStampsCompany(t *testing.T) {
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

	const company = "Braintrust"
	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	res, err := c.FetchBoard(context.Background(), "braintrust", company, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 1 {
		t.Fatalf("jobs=%d", len(res.Jobs))
	}
	j := res.Jobs[0]
	if j.Company != company {
		t.Fatalf("company=%q", j.Company)
	}
	wantKey := domainutils.CompanyKey(company)
	if j.CompanyKey != wantKey || wantKey == "" {
		t.Fatalf("companyKey=%q want %q", j.CompanyKey, wantKey)
	}
	if strings.Contains(j.ID, "http") || strings.Contains(j.ID, j.URL) {
		t.Fatalf("id must not contain listing URL: %q", j.ID)
	}
	wantID := domainutils.StableJobID(wantKey, j.Title)
	if j.ID != wantID {
		t.Fatalf("id=%q want %q", j.ID, wantID)
	}
}

func TestFetchBoard_sameTitleDifferentURLsShareStableID(t *testing.T) {
	t.Parallel()
	body := `{
  "jobs": [
    {
      "id": "loc-a",
      "title": "Sales Development Representative",
      "location": "San Francisco",
      "publishedAt": "2026-10-13T13:48:21.468+00:00",
      "isListed": true,
      "isRemote": false,
      "workplaceType": "On-site",
      "jobUrl": "https://jobs.ashbyhq.com/braintrust/loc-a",
      "applyUrl": "https://jobs.ashbyhq.com/braintrust/loc-a/application",
      "descriptionPlain": "SDR role."
    },
    {
      "id": "loc-b",
      "title": "Sales Development Representative",
      "location": "New York",
      "publishedAt": "2026-10-13T13:48:21.468+00:00",
      "isListed": true,
      "isRemote": false,
      "workplaceType": "On-site",
      "jobUrl": "https://jobs.ashbyhq.com/braintrust/loc-b",
      "applyUrl": "https://jobs.ashbyhq.com/braintrust/loc-b/application",
      "descriptionPlain": "SDR role."
    }
  ],
  "apiVersion": "1"
}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	res, err := c.FetchBoard(context.Background(), "braintrust", "braintrust.dev", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 2 {
		t.Fatalf("jobs=%d", len(res.Jobs))
	}
	if res.Jobs[0].URL == res.Jobs[1].URL {
		t.Fatal("expected different listing URLs")
	}
	if res.Jobs[0].ID != res.Jobs[1].ID {
		t.Fatalf("stable IDs differ: %q vs %q", res.Jobs[0].ID, res.Jobs[1].ID)
	}
	if strings.Contains(res.Jobs[0].ID, "http") {
		t.Fatalf("placeholder id leaked URL: %q", res.Jobs[0].ID)
	}
}

func TestFetchBoard_emptyCompanyName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("must not hit network with empty companyName")
	}))
	t.Cleanup(srv.Close)
	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	res, err := c.FetchBoard(context.Background(), "n8n", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, companies.ErrBoardFormat) {
		t.Fatalf("err=%v want ErrBoardFormat", err)
	}
	if len(res.Jobs) != 0 {
		t.Fatalf("jobs=%d", len(res.Jobs))
	}
}

func TestFetchBoard_slugVerbatim(t *testing.T) {
	t.Parallel()
	for _, slug := range []string{"Coder", "Deepnote", "primer.io", "ecosia.org"} {
		slug := slug
		t.Run(slug, func(t *testing.T) {
			t.Parallel()
			var path string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path = r.URL.Path
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, "Not Found")
			}))
			t.Cleanup(srv.Close)
			c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
			res, err := c.FetchBoard(context.Background(), slug, "Acme", "")
			if err != nil {
				t.Fatal(err)
			}
			if res.Outcome != companyschema.PollOutcomeDeadSlug {
				t.Fatalf("outcome=%s", res.Outcome)
			}
			want := "/posting-api/job-board/" + slug
			if path != want {
				t.Fatalf("path=%q want %q", path, want)
			}
		})
	}
}

func TestFetchBoard_404PlainTextNotFormatError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "Not Found")
	}))
	t.Cleanup(srv.Close)
	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	res, err := c.FetchBoard(context.Background(), "missing", "Acme", "")
	if err != nil {
		t.Fatalf("must not decode 404 body: %v", err)
	}
	if res.Outcome != companyschema.PollOutcomeDeadSlug {
		t.Fatalf("outcome=%s", res.Outcome)
	}
}

func TestFetchBoard_mapsSecondaryLocations(t *testing.T) {
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
	res, err := c.FetchBoard(context.Background(), "n8n", "n8n", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != companyschema.PollOutcomeComplete || len(res.Jobs) != 1 {
		t.Fatalf("got outcome=%s jobs=%d", res.Outcome, len(res.Jobs))
	}
	raw := res.Jobs[0].Location.Raw
	if !strings.Contains(raw, "Bosnia") && !strings.Contains(raw, "Norway") && !strings.Contains(raw, "Berlin") {
		t.Fatalf("raw location missing secondary hints: %q", raw)
	}
}
