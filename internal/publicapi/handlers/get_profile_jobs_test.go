package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/rs/zerolog"
)

func TestGetProfileJobs(t *testing.T) {
	seen := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	posted := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	t.Run("bad query", func(t *testing.T) {
		for _, rawURL := range []string{
			"/api/v1/profiles/software/jobs?bucket=PASSED",
			"/api/v1/profiles/software/jobs?bucket=BOTH",
			"/api/v1/profiles/software/jobs?user_status=ALL",
			"/api/v1/profiles/software/jobs?user_status=NEW&user_status=HIDDEN",
			"/api/v1/profiles/software/jobs?closed=yes",
			"/api/v1/profiles/software/jobs?page=0",
			"/api/v1/profiles/software/jobs?limit=101",
			"/api/v1/profiles/software/jobs?page=no",
		} {
			t.Run(rawURL, func(t *testing.T) {
				api := &profileJobsScoring{}
				h := NewHTTPHandler(nil, Deps{
					Logger:   zerolog.Nop(),
					Profiles: profileJobsStore{},
					Scoring:  api,
				})
				req := httptest.NewRequest(http.MethodGet, rawURL, nil)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status %d %s", rec.Code, rec.Body.String())
				}
				var body schema.APIErrorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				assertAPIError(t, body.Error, schema.APIErrorCodeInvalidQuery)
				if api.listCalled {
					t.Fatal("ListJobs called")
				}
			})
		}
	})

	t.Run("default top and new", func(t *testing.T) {
		api := &profileJobsScoring{page: scoringschema.JobPage{
			Page:  1,
			Limit: schema.DefaultJobListLimit,
			Total: 1,
			Jobs: []scoringschema.ListedJob{{
				JobID:       "job-1",
				Title:       "Backend",
				Company:     "Acme",
				URL:         "https://example.com/job",
				ApplyURL:    "https://example.com/apply",
				Location:    jobdata.Location{Type: jobdata.LocationRemote, Raw: "Remote"},
				Sources:     []string{"himalayas"},
				PostedAt:    posted,
				FirstSeenAt: seen,
				LastSeenAt:  seen,
				Bucket:      scoringschema.BucketTop,
				Score:       0,
				Signals: []scoringschema.Signal{{
					Code:        scoringschema.SignalPlus,
					Rule:        "signals_plus.title",
					Phrase:      "golang",
					Delta:       1,
					Occurrences: 1,
				}},
				UserStatus: scoringschema.UserStatusNew,
			}},
		}}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/software/jobs?min_score=9", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if api.listParams.ProfileID != "software" ||
			api.listParams.Bucket != scoringschema.BucketTop ||
			api.listParams.UserStatus != scoringschema.UserStatusNew ||
			api.listParams.Page != 1 ||
			api.listParams.Limit != schema.DefaultJobListLimit ||
			api.listParams.IncludeClosed {
			t.Fatalf("params %#v", api.listParams)
		}
		if strings.Contains(rec.Body.String(), "min_score") {
			t.Fatalf("score threshold in body %s", rec.Body.String())
		}
		var got schema.ProfileJobPage
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Page != 1 || got.Limit != schema.DefaultJobListLimit || got.Total != 1 || len(got.Jobs) != 1 {
			t.Fatalf("page %#v", got)
		}
		job := got.Jobs[0]
		if job.JobID != "job-1" || job.Bucket != "TOP" || job.Score != 0 || job.UserStatus != "NEW" {
			t.Fatalf("job %#v", job)
		}
		if job.Location.Type != "remote" || job.Location.Raw != "Remote" || len(job.Location.Regions) != 0 || len(job.Location.Countries) != 0 {
			t.Fatalf("location %#v", job.Location)
		}
		if len(job.Sources) != 1 || job.Sources[0] != "himalayas" {
			t.Fatalf("sources %#v", job.Sources)
		}
		if len(job.Signals) != 1 || job.Signals[0].Code != "PLUS" || job.Signals[0].Rule != "signals_plus.title" || job.Signals[0].Phrase != "golang" || job.Signals[0].Delta != 1 || job.Signals[0].Occurrences != 1 || job.Signals[0].Value != "" {
			t.Fatalf("signals %#v", job.Signals)
		}
		if strings.Contains(rec.Body.String(), `"value"`) || strings.Contains(rec.Body.String(), `"points"`) || strings.Contains(rec.Body.String(), `"terms"`) {
			t.Fatalf("old signal fields in body %s", rec.Body.String())
		}
		if job.PostedAt == nil || !job.PostedAt.Equal(posted) || !job.FirstSeenAt.Equal(seen) || !job.LastSeenAt.Equal(seen) {
			t.Fatalf("times posted %v first %s last %s", job.PostedAt, job.FirstSeenAt, job.LastSeenAt)
		}
		if job.Openings == nil || len(job.Openings) != 0 {
			t.Fatalf("openings %#v", job.Openings)
		}
	})

	t.Run("two openings", func(t *testing.T) {
		api := &profileJobsScoring{page: scoringschema.JobPage{
			Page:  1,
			Limit: schema.DefaultJobListLimit,
			Total: 1,
			Jobs: []scoringschema.ListedJob{{
				JobID:    "job-1",
				Title:    "Backend",
				Company:  "Acme",
				URL:      "https://example.com/us",
				ApplyURL: "https://example.com/us/apply",
				Location: jobdata.Location{
					Type:      jobdata.LocationRemote,
					Countries: []string{"GR", "US"},
					Raw:       "Greece, United States",
				},
				Openings: []jobdata.Opening{
					{Country: "United States", URL: "https://example.com/us", ApplyURL: "https://example.com/us/apply"},
					{Country: "Greece", URL: "https://example.com/gr", ApplyURL: ""},
				},
				FirstSeenAt: seen,
				LastSeenAt:  seen,
				Bucket:      scoringschema.BucketTop,
				UserStatus:  scoringschema.UserStatusNew,
			}},
		}}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/software/jobs", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		var got schema.ProfileJobPage
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Jobs) != 1 {
			t.Fatalf("jobs %#v", got.Jobs)
		}
		job := got.Jobs[0]
		if job.URL != "https://example.com/us" || job.ApplyURL != "https://example.com/us/apply" {
			t.Fatalf("urls %#v", job)
		}
		if job.Location.Type != "remote" || job.Location.Raw != "Greece, United States" || len(job.Location.Countries) != 2 || job.Location.Countries[0] != "GR" || job.Location.Countries[1] != "US" {
			t.Fatalf("location %#v", job.Location)
		}
		if len(job.Openings) != 2 ||
			job.Openings[0].Country != "United States" || job.Openings[0].URL != "https://example.com/us" || job.Openings[0].ApplyURL != "https://example.com/us/apply" ||
			job.Openings[1].Country != "Greece" || job.Openings[1].URL != "https://example.com/gr" || job.Openings[1].ApplyURL != "" {
			t.Fatalf("openings %#v", job.Openings)
		}
	})

	t.Run("review bucket", func(t *testing.T) {
		api := &profileJobsScoring{page: scoringschema.JobPage{
			Page:  1,
			Limit: schema.DefaultJobListLimit,
		}}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/software/jobs?bucket=REVIEW", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if api.listParams.Bucket != scoringschema.BucketReview {
			t.Fatalf("params %#v", api.listParams)
		}
	})

	t.Run("rejected page", func(t *testing.T) {
		api := &profileJobsScoring{page: scoringschema.JobPage{
			Page:  2,
			Limit: 25,
			Total: 3,
			Jobs: []scoringschema.ListedJob{{
				JobID:       "job-2",
				Title:       "Intern",
				Company:     "Acme",
				URL:         "https://example.com/intern",
				ApplyURL:    "https://example.com/intern/apply",
				Location:    jobdata.Location{Type: jobdata.LocationOffice, Raw: "Berlin"},
				FirstSeenAt: seen,
				LastSeenAt:  seen,
				Bucket:      scoringschema.BucketRejected,
				Score:       -10,
				Signals: []scoringschema.Signal{{
					Code:        scoringschema.SignalGateRole,
					Rule:        "gate_role",
					Phrase:      "intern",
					Delta:       0,
					Occurrences: 1,
				}},
				UserStatus: scoringschema.UserStatusNew,
			}},
		}}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/software/jobs?bucket=REJECTED&page=2&limit=25", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if api.listParams.Bucket != scoringschema.BucketRejected ||
			api.listParams.UserStatus != scoringschema.UserStatusNew ||
			api.listParams.Page != 2 ||
			api.listParams.Limit != 25 {
			t.Fatalf("params %#v", api.listParams)
		}
		var got schema.ProfileJobPage
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Page != 2 || got.Limit != 25 || got.Total != 3 || len(got.Jobs) != 1 {
			t.Fatalf("page %#v", got)
		}
		job := got.Jobs[0]
		if job.Bucket != "REJECTED" || job.Score != -10 || job.UserStatus != "NEW" || job.PostedAt != nil {
			t.Fatalf("job %#v", job)
		}
		if job.Sources == nil || job.Signals == nil || len(job.Signals) != 1 || job.Signals[0].Code != "GATE_ROLE" || job.Signals[0].Rule != "gate_role" || job.Signals[0].Phrase != "intern" || job.Signals[0].Delta != 0 || job.Signals[0].Occurrences != 1 || job.Signals[0].Value != "" {
			t.Fatalf("lists sources %#v signals %#v", job.Sources, job.Signals)
		}
		if strings.Contains(rec.Body.String(), `"value"`) {
			t.Fatalf("phrase signal value in body %s", rec.Body.String())
		}
	})

	t.Run("facts ready INR gate", func(t *testing.T) {
		api := &profileJobsScoring{page: scoringschema.JobPage{
			Page:  1,
			Limit: schema.DefaultJobListLimit,
			Total: 1,
			Jobs: []scoringschema.ListedJob{{
				JobID:       "job-inr",
				Title:       "Backend",
				Company:     "Acme",
				URL:         "https://example.com/inr",
				ApplyURL:    "https://example.com/inr/apply",
				Location:    jobdata.Location{Type: jobdata.LocationRemote, Raw: "Remote"},
				FirstSeenAt: seen,
				LastSeenAt:  seen,
				Bucket:      scoringschema.BucketRejected,
				Score:       0,
				FactsStatus: "ready",
				Signals: []scoringschema.Signal{{
					Code:   scoringschema.SignalGateHard,
					Rule:   "facts.compensation",
					Value:  "INR",
					Phrase: "25 LPA",
					Delta:  0,
				}},
				UserStatus: scoringschema.UserStatusNew,
			}},
		}}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/software/jobs?bucket=REJECTED", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		var got schema.ProfileJobPage
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Jobs) != 1 {
			t.Fatalf("jobs %#v", got.Jobs)
		}
		job := got.Jobs[0]
		if job.FactsStatus != "ready" {
			t.Fatalf("facts_status %q", job.FactsStatus)
		}
		if len(job.Signals) != 1 ||
			job.Signals[0].Code != "GATE_HARD" ||
			job.Signals[0].Rule != "facts.compensation" ||
			job.Signals[0].Value != "INR" ||
			job.Signals[0].Phrase != "25 LPA" ||
			job.Signals[0].Delta != 0 {
			t.Fatalf("signals %#v", job.Signals)
		}
	})

	t.Run("unknown profile", func(t *testing.T) {
		api := &profileJobsScoring{}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{err: profiles.ErrProfileNotFound},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/missing/jobs", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		var body schema.APIErrorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		assertAPIError(t, body.Error, schema.APIErrorCodeProfileNotFound)
		if api.listCalled {
			t.Fatal("ListJobs called")
		}
	})

	t.Run("closed excluded by default", func(t *testing.T) {
		api := &profileJobsScoring{page: scoringschema.JobPage{
			Page:  1,
			Limit: schema.DefaultJobListLimit,
		}}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/software/jobs", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if api.listParams.IncludeClosed {
			t.Fatalf("IncludeClosed want false, got %#v", api.listParams)
		}
	})

	t.Run("closed=true includes closed and returns closed_at", func(t *testing.T) {
		closedAt := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
		api := &profileJobsScoring{page: scoringschema.JobPage{
			Page:  1,
			Limit: schema.DefaultJobListLimit,
			Total: 1,
			Jobs: []scoringschema.ListedJob{{
				JobID:       "job-closed",
				Title:       "Backend",
				Company:     "Acme",
				URL:         "https://example.com/closed",
				ApplyURL:    "https://example.com/closed/apply",
				Location:    jobdata.Location{Type: jobdata.LocationRemote, Raw: "Remote"},
				FirstSeenAt: seen,
				LastSeenAt:  seen,
				ClosedAt:    &closedAt,
				Bucket:      scoringschema.BucketTop,
				UserStatus:  scoringschema.UserStatusNew,
			}},
		}}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/software/jobs?closed=true", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if !api.listParams.IncludeClosed {
			t.Fatalf("IncludeClosed want true, got %#v", api.listParams)
		}
		var got schema.ProfileJobPage
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Jobs) != 1 || got.Jobs[0].ClosedAt == nil || !got.Jobs[0].ClosedAt.Equal(closedAt) {
			t.Fatalf("jobs %#v", got.Jobs)
		}
	})
}

type profileJobsStore struct {
	err error
}

func (s profileJobsStore) List(context.Context) ([]profileschema.Profile, error) {
	return nil, nil
}

func (s profileJobsStore) Get(context.Context, string) (profileschema.Profile, error) {
	if s.err != nil {
		return profileschema.Profile{}, s.err
	}
	return profileschema.Profile{ID: "software"}, nil
}

type profileJobsScoring struct {
	page       scoringschema.JobPage
	listCalled bool
	listParams scoringschema.ListJobsParams
}

func (f *profileJobsScoring) StartRun(context.Context, string, uuid.UUID) (scoringschema.Run, error) {
	return scoringschema.Run{}, nil
}

func (f *profileJobsScoring) LatestRun(context.Context, string) (scoringschema.Run, error) {
	return scoringschema.Run{}, nil
}

func (f *profileJobsScoring) ListJobs(_ context.Context, p scoringschema.ListJobsParams) (scoringschema.JobPage, error) {
	f.listCalled = true
	f.listParams = p
	return f.page, nil
}

func (f *profileJobsScoring) SetUserStatus(context.Context, string, string, scoringschema.UserStatus) (scoringschema.ListedJob, error) {
	return scoringschema.ListedJob{}, nil
}
