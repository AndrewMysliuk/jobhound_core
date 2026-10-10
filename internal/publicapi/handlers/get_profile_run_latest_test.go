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

	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/rs/zerolog"
)

func TestGetProfileRunLatest(t *testing.T) {
	started := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	finished := started.Add(time.Minute)

	tests := []struct {
		name       string
		profileErr error
		run        scoringschema.Run
		runErr     error
		wantStatus int
		wantCode   schema.APIErrorCode
		wantCalled bool
	}{
		{
			name: "ok",
			run: scoringschema.Run{
				ID:             7,
				ProfileID:      "andrew",
				Status:         scoringschema.RunStatusSucceeded,
				StartedAt:      started,
				FinishedAt:     &finished,
				JobsScored:     4,
				SourcesSkipped: 1,
			},
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
		{
			name:       "unknown profile",
			profileErr: profiles.ErrProfileNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   schema.APIErrorCodeProfileNotFound,
		},
		{
			name:       "no run",
			runErr:     scoring.ErrNoRun,
			wantStatus: http.StatusNotFound,
			wantCode:   schema.APIErrorCodeNoRun,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeScoringAPI{latest: tt.run, latestErr: tt.runErr}
			h := NewHTTPHandler(nil, Deps{
				Logger:   zerolog.Nop(),
				Profiles: latestProfileStore{err: tt.profileErr},
				Scoring:  api,
			})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/andrew/runs/latest", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status %d %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("content-type %q", rec.Header().Get("Content-Type"))
			}
			if api.called != tt.wantCalled {
				t.Fatalf("LatestRun called %v", api.called)
			}
			if tt.wantCalled && api.gotID != "andrew" {
				t.Fatalf("profile id %q", api.gotID)
			}
			if tt.wantCode != "" {
				var body schema.APIErrorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				assertAPIError(t, body.Error, tt.wantCode)
				return
			}
			var got schema.ProfileRunResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.RunID != 7 || got.ProfileID != "andrew" || got.Status != "SUCCEEDED" || got.JobsScored != 4 || got.SourcesSkipped != 1 {
				t.Fatalf("body %#v", got)
			}
			if !got.StartedAt.Equal(started) || got.FinishedAt == nil || !got.FinishedAt.Equal(finished) {
				t.Fatalf("times start %s finish %v", got.StartedAt, got.FinishedAt)
			}
		})
	}
}

type latestProfileStore struct {
	err error
}

func (s latestProfileStore) List(context.Context) ([]profileschema.Profile, error) {
	return nil, nil
}

func (s latestProfileStore) Get(context.Context, string) (profileschema.Profile, error) {
	if s.err != nil {
		return profileschema.Profile{}, s.err
	}
	return profileschema.Profile{ID: "andrew"}, nil
}

type fakeScoringAPI struct {
	latest    scoringschema.Run
	latestErr error
	called    bool
	gotID     string
}

func (f *fakeScoringAPI) StartRun(context.Context, string, uuid.UUID) (scoringschema.Run, error) {
	return scoringschema.Run{}, nil
}

func (f *fakeScoringAPI) LatestRun(_ context.Context, profileID string) (scoringschema.Run, error) {
	f.called = true
	f.gotID = profileID
	return f.latest, f.latestErr
}

func (f *fakeScoringAPI) ListJobs(context.Context, scoringschema.ListJobsParams) (scoringschema.JobPage, error) {
	return scoringschema.JobPage{}, nil
}

func (f *fakeScoringAPI) SetUserStatus(context.Context, string, string, scoringschema.UserStatus) (scoringschema.ListedJob, error) {
	return scoringschema.ListedJob{}, nil
}
