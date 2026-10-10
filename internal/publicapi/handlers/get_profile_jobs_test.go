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
			"/api/v1/profiles/andrew/jobs?bucket=BOTH",
			"/api/v1/profiles/andrew/jobs?user_status=ALL",
			"/api/v1/profiles/andrew/jobs?user_status=NEW&user_status=HIDDEN",
			"/api/v1/profiles/andrew/jobs?page=0",
			"/api/v1/profiles/andrew/jobs?limit=101",
			"/api/v1/profiles/andrew/jobs?page=no",
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

	t.Run("default passed and new", func(t *testing.T) {
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
				Bucket:      scoringschema.BucketPassed,
				Score:       0,
				Signals:     []scoringschema.Signal{{Code: scoringschema.SignalQuery, Points: 1, Terms: []string{"golang"}}},
				UserStatus:  scoringschema.UserStatusNew,
			}},
		}}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/andrew/jobs?min_score=9", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if api.listParams.ProfileID != "andrew" ||
			api.listParams.Bucket != scoringschema.BucketPassed ||
			api.listParams.UserStatus != scoringschema.UserStatusNew ||
			api.listParams.Page != 1 ||
			api.listParams.Limit != schema.DefaultJobListLimit {
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
		if job.JobID != "job-1" || job.Bucket != "PASSED" || job.Score != 0 || job.UserStatus != "NEW" {
			t.Fatalf("job %#v", job)
		}
		if job.Location.Type != "remote" || job.Location.Raw != "Remote" || len(job.Location.Regions) != 0 || len(job.Location.Countries) != 0 {
			t.Fatalf("location %#v", job.Location)
		}
		if len(job.Sources) != 1 || job.Sources[0] != "himalayas" {
			t.Fatalf("sources %#v", job.Sources)
		}
		if len(job.Signals) != 1 || job.Signals[0].Code != "QUERY" || job.Signals[0].Points != 1 || len(job.Signals[0].Terms) != 1 || job.Signals[0].Terms[0] != "golang" {
			t.Fatalf("signals %#v", job.Signals)
		}
		if job.PostedAt == nil || !job.PostedAt.Equal(posted) || !job.FirstSeenAt.Equal(seen) || !job.LastSeenAt.Equal(seen) {
			t.Fatalf("times posted %v first %s last %s", job.PostedAt, job.FirstSeenAt, job.LastSeenAt)
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
				Signals:     []scoringschema.Signal{{Code: scoringschema.SignalCutExcludeTitle, Points: 0, Terms: []string{"intern"}}},
				UserStatus:  scoringschema.UserStatusNew,
			}},
		}}
		h := NewHTTPHandler(nil, Deps{
			Logger:   zerolog.Nop(),
			Profiles: profileJobsStore{},
			Scoring:  api,
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/andrew/jobs?bucket=REJECTED&page=2&limit=25", nil)
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
		if job.Sources == nil || job.Signals == nil || len(job.Signals) != 1 || job.Signals[0].Code != "CUT_EXCLUDE_TITLE" {
			t.Fatalf("lists sources %#v signals %#v", job.Sources, job.Signals)
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
	return profileschema.Profile{ID: "andrew"}, nil
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
