package lever

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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
		etagIn   string
		pageSize int
		maxPages int
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
			company:  "Lever Demo",
		},
		{
			name:     "empty",
			status:   http.StatusOK,
			bodyFile: "jobs_empty.json",
			wantOut:  companyschema.PollOutcomeEmpty,
			company:  "Lever Demo",
		},
		{
			name:     "dead_slug",
			status:   http.StatusNotFound,
			bodyFile: "jobs_404.json",
			wantOut:  companyschema.PollOutcomeDeadSlug,
			company:  "Lever Demo",
		},
		{
			name:     "not_modified",
			status:   http.StatusNotModified,
			etagIn:   `"prev"`,
			wantOut:  companyschema.PollOutcomeNotModified,
			wantETag: `"prev"`,
			company:  "Lever Demo",
		},
		{
			name:     "partial_max_pages",
			status:   http.StatusOK,
			bodyFile: "jobs_two.json",
			pageSize: 1,
			maxPages: 1,
			wantOut:  companyschema.PollOutcomePartial,
			wantJobs: 1,
			company:  "Lever Demo",
		},
		{
			name:     "partial_max_jobs",
			status:   http.StatusOK,
			bodyFile: "jobs_two.json",
			maxJobs:  1,
			wantOut:  companyschema.PollOutcomePartial,
			wantJobs: 1,
			company:  "Lever Demo",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotINM, gotPath, gotQuery string
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				gotINM = r.Header.Get("If-None-Match")
				gotPath = r.URL.Path
				gotQuery = r.URL.RawQuery
				if tc.wantETag != "" && tc.status == http.StatusOK && calls == 1 {
					w.Header().Set("ETag", tc.wantETag)
				}
				w.WriteHeader(tc.status)
				if tc.bodyFile == "" {
					return
				}
				b, err := os.ReadFile(filepath.Join("testdata", tc.bodyFile))
				if err != nil {
					t.Fatalf("fixture: %v", err)
				}
				if tc.pageSize == 1 && tc.bodyFile == "jobs_two.json" {
					// Serve one job per page based on skip.
					skip, _ := strconv.Atoi(r.URL.Query().Get("skip"))
					all := mustDecodeJobs(t, b)
					if skip >= len(all) {
						_, _ = io.WriteString(w, "[]")
						return
					}
					one, err := marshalOne(all[skip])
					if err != nil {
						t.Fatal(err)
					}
					_, _ = w.Write(one)
					return
				}
				_, _ = w.Write(b)
			}))
			t.Cleanup(srv.Close)

			c := &Client{
				HTTPClient: srv.Client(),
				BaseURL:    srv.URL,
				PageSize:   tc.pageSize,
				MaxPages:   tc.maxPages,
				MaxJobs:    tc.maxJobs,
			}
			res, err := c.FetchBoard(context.Background(), "leverdemo", tc.company, tc.etagIn)
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
				t.Fatalf("If-None-Match=%q", gotINM)
			}
			if tc.wantETag != "" && res.ETag != tc.wantETag {
				t.Fatalf("etag=%q want %q", res.ETag, tc.wantETag)
			}
			if !strings.Contains(gotQuery, "mode=json") {
				t.Fatalf("query missing mode=json: %q", gotQuery)
			}
			if gotPath != "/v0/postings/leverdemo" {
				t.Fatalf("path=%q", gotPath)
			}
			if tc.wantJobs > 0 {
				assertMapped(t, res.Jobs[0], tc.company)
			}
		})
	}
}

func assertMapped(t *testing.T, j domainschema.Job, company string) {
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
	if j.ApplyURL == "" || j.URL == "" {
		t.Fatal("missing urls")
	}
	if j.Location.Type != domainschema.LocationHybrid {
		t.Fatalf("location type=%q want hybrid", j.Location.Type)
	}
	if j.PostedAt.IsZero() {
		t.Fatal("expected createdAt mapped")
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

	const company = "Lever Demo"
	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	res, err := c.FetchBoard(context.Background(), "leverdemo", company, "")
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
}

func TestFetchBoard_sameTitleDifferentURLsShareStableID(t *testing.T) {
	t.Parallel()
	body := `[
  {
    "id": "loc-a",
    "text": "Sales Development Representative",
    "categories": {"location": "San Francisco", "allLocations": ["San Francisco"]},
    "country": "US",
    "createdAt": 1788199939703,
    "descriptionPlain": "SDR role.",
    "hostedUrl": "https://jobs.lever.co/braintrust/loc-a",
    "applyUrl": "https://jobs.lever.co/braintrust/loc-a/apply",
    "workplaceType": "onsite"
  },
  {
    "id": "loc-b",
    "text": "Sales Development Representative",
    "categories": {"location": "New York", "allLocations": ["New York"]},
    "country": "US",
    "createdAt": 1788199939703,
    "descriptionPlain": "SDR role.",
    "hostedUrl": "https://jobs.lever.co/braintrust/loc-b",
    "applyUrl": "https://jobs.lever.co/braintrust/loc-b/apply",
    "workplaceType": "onsite"
  }
]`
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
	res, err := c.FetchBoard(context.Background(), "leverdemo", "  ", "")
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
	for _, slug := range []string{"primer.io", "Coder", "541"} {
		slug := slug
		t.Run(slug, func(t *testing.T) {
			t.Parallel()
			var path string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path = r.URL.Path
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"ok":false,"error":"Document not found"}`)
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
			want := "/v0/postings/" + slug
			if path != want {
				t.Fatalf("path=%q want %q", path, want)
			}
		})
	}
}

func mustDecodeJobs(t *testing.T, b []byte) []jobWire {
	t.Helper()
	var jobs []jobWire
	if err := json.Unmarshal(b, &jobs); err != nil {
		t.Fatal(err)
	}
	return jobs
}

func marshalOne(j jobWire) ([]byte, error) {
	return json.Marshal([]jobWire{j})
}
