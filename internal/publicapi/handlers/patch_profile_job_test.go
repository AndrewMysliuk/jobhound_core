package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestPatchProfileJob(t *testing.T) {
	posted := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	seen := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	stored := scoringschema.ListedJob{
		JobID:    "job-1",
		Title:    "Vue engineer",
		Company:  "Acme",
		URL:      "https://example.com/jobs/1",
		ApplyURL: "https://ats.example/apply",
		Location: jobdata.Location{
			Type:      "remote",
			Regions:   []string{"europe"},
			Countries: []string{"DE"},
			Timezone:  "CET",
			Raw:       "Remote",
		},
		Sources:     []string{"vue_jobs"},
		PostedAt:    posted,
		FirstSeenAt: seen,
		LastSeenAt:  seen,
		Bucket:      scoringschema.BucketPassed,
		Score:       1,
		Signals: []scoringschema.Signal{{
			Code:   scoringschema.SignalQuery,
			Points: 1,
			Terms:  []string{"vue"},
		}},
		UserStatus: scoringschema.UserStatusNew,
	}

	t.Run("200", func(t *testing.T) {
		api := &patchScoring{job: stored}
		h := patchHandler(api, nil)
		rec := patchRequest(t, h, "/api/v1/profiles/andrew/jobs/job-1", `{"user_status":"HIDDEN"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if api.profileID != "andrew" || api.jobID != "job-1" || api.status != scoringschema.UserStatusHidden {
			t.Fatalf("set status call profile=%q job=%q status=%q", api.profileID, api.jobID, api.status)
		}
		var got schema.ProfileJob
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.JobID != "job-1" || got.Title != "Vue engineer" || got.Company != "Acme" {
			t.Fatalf("job %#v", got)
		}
		if got.URL != stored.URL || got.ApplyURL != stored.ApplyURL {
			t.Fatalf("urls %#v", got)
		}
		if got.Location.Type != "remote" || got.Location.Raw != "Remote" || len(got.Location.Regions) != 1 || got.Location.Regions[0] != "europe" {
			t.Fatalf("location %#v", got.Location)
		}
		if len(got.Sources) != 1 || got.Sources[0] != "vue_jobs" {
			t.Fatalf("sources %#v", got.Sources)
		}
		if got.PostedAt == nil || !got.PostedAt.Equal(posted) || !got.FirstSeenAt.Equal(seen) || !got.LastSeenAt.Equal(seen) {
			t.Fatalf("times posted %v first %s last %s", got.PostedAt, got.FirstSeenAt, got.LastSeenAt)
		}
		if got.Bucket != scoringschema.BucketPassed.String() || got.Score != 1 || got.UserStatus != scoringschema.UserStatusHidden.String() {
			t.Fatalf("score %#v", got)
		}
		if len(got.Signals) != 1 || got.Signals[0].Code != scoringschema.SignalQuery.String() || got.Signals[0].Points != 1 {
			t.Fatalf("signals %#v", got.Signals)
		}
	})

	t.Run("404 unknown profile", func(t *testing.T) {
		api := &patchScoring{job: stored}
		h := patchHandler(api, profiles.ErrProfileNotFound)
		rec := patchRequest(t, h, "/api/v1/profiles/missing/jobs/job-1", `{"user_status":"HIDDEN"}`)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		var body schema.APIErrorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		assertAPIError(t, body.Error, schema.APIErrorCodeProfileNotFound)
		if api.calls != 0 {
			t.Fatalf("scoring calls %d", api.calls)
		}
	})

	t.Run("404 job not in scope", func(t *testing.T) {
		api := &patchScoring{err: scoring.ErrJobNotInScope}
		h := patchHandler(api, nil)
		rec := patchRequest(t, h, "/api/v1/profiles/andrew/jobs/job-1", `{"user_status":"NEW"}`)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		var body schema.APIErrorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		assertAPIError(t, body.Error, schema.APIErrorCodeJobNotInScope)
		if api.calls != 1 || api.status != scoringschema.UserStatusNew {
			t.Fatalf("calls %d status %q", api.calls, api.status)
		}
	})

	t.Run("400 invalid status", func(t *testing.T) {
		api := &patchScoring{job: stored}
		h := patchHandler(api, nil)
		rec := patchRequest(t, h, "/api/v1/profiles/andrew/jobs/job-1", `{"user_status":"SAVED"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		var body schema.APIErrorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		assertAPIError(t, body.Error, schema.APIErrorCodeValidationFailed)
		if api.calls != 0 {
			t.Fatalf("scoring calls %d", api.calls)
		}
	})
}

func patchHandler(api *patchScoring, profileErr error) *HTTPHandler {
	return NewHTTPHandler(nil, Deps{
		Logger:   zerolog.Nop(),
		Profiles: patchProfileStore{err: profileErr},
		Scoring:  api,
	})
}

func patchRequest(t *testing.T, h *HTTPHandler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type patchProfileStore struct {
	err error
}

func (s patchProfileStore) List(context.Context) ([]profileschema.Profile, error) {
	return nil, nil
}

func (s patchProfileStore) Get(context.Context, string) (profileschema.Profile, error) {
	if s.err != nil {
		return profileschema.Profile{}, s.err
	}
	return profileschema.Profile{ID: "andrew"}, nil
}

type patchScoring struct {
	job       scoringschema.ListedJob
	err       error
	calls     int
	profileID string
	jobID     string
	status    scoringschema.UserStatus
}

func (f *patchScoring) StartRun(context.Context, string, uuid.UUID) (scoringschema.Run, error) {
	return scoringschema.Run{}, nil
}

func (f *patchScoring) LatestRun(context.Context, string) (scoringschema.Run, error) {
	return scoringschema.Run{}, scoring.ErrNoRun
}

func (f *patchScoring) ListJobs(context.Context, scoringschema.ListJobsParams) (scoringschema.JobPage, error) {
	return scoringschema.JobPage{}, nil
}

func (f *patchScoring) SetUserStatus(_ context.Context, profileID, jobID string, status scoringschema.UserStatus) (scoringschema.ListedJob, error) {
	f.calls++
	f.profileID = profileID
	f.jobID = jobID
	f.status = status
	if f.err != nil {
		return scoringschema.ListedJob{}, f.err
	}
	job := f.job
	job.UserStatus = status
	return job, nil
}
