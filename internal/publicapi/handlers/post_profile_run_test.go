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
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	"github.com/rs/zerolog"
)

func TestPostProfileRun(t *testing.T) {
	key := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	started := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	run := scoringschema.Run{
		ID:        9,
		ProfileID: "andrew",
		Status:    scoringschema.RunStatusRunning,
		StartedAt: started,
	}

	tests := []struct {
		name      string
		key       string
		omitKey   bool
		body      string
		err       error
		want      int
		wantCode  schema.APIErrorCode
		callStart bool
		replay    bool
	}{
		{name: "accepted", key: key, want: http.StatusAccepted, callStart: true, replay: true},
		{name: "already running", key: key, err: scoring.ErrRunAlreadyRunning, want: http.StatusConflict, wantCode: schema.APIErrorCodeRunAlreadyRunning, callStart: true},
		{name: "key reused", key: key, err: scoring.ErrIdempotencyKeyConflict, want: http.StatusConflict, wantCode: schema.APIErrorCodeIdempotencyKeyConflict, callStart: true},
		{name: "missing key", omitKey: true, want: http.StatusBadRequest, wantCode: schema.APIErrorCodeIdempotencyKeyRequired},
		{name: "invalid key", key: "not-a-uuid", want: http.StatusBadRequest, wantCode: schema.APIErrorCodeInvalidIdempotencyKey},
		{name: "nil key", key: uuid.Nil.String(), want: http.StatusBadRequest, wantCode: schema.APIErrorCodeInvalidIdempotencyKey},
		{name: "json body", key: key, body: `{"queries":["golang"]}`, want: http.StatusBadRequest, wantCode: schema.APIErrorCodeValidationFailed},
		{name: "unknown profile", key: key, err: profiles.ErrProfileNotFound, want: http.StatusNotFound, wantCode: schema.APIErrorCodeProfileNotFound, callStart: true},
		{name: "invalid definition", key: key, err: profiles.ErrProfileInvalid, want: http.StatusBadRequest, wantCode: schema.APIErrorCodeProfileInvalidDefinition, callStart: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &profileRunScoring{run: run, err: tt.err}
			h := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Scoring: api})
			rec := postProfileRunReq(h, tt.key, tt.omitKey, tt.body)
			if rec.Code != tt.want {
				t.Fatalf("status %d %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("content-type %q", rec.Header().Get("Content-Type"))
			}
			if tt.wantCode != "" {
				var body schema.APIErrorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				assertAPIError(t, body.Error, tt.wantCode)
				wantCalls := 0
				if tt.callStart {
					wantCalls = 1
				}
				if api.calls != wantCalls {
					t.Fatalf("StartRun calls %d", api.calls)
				}
				return
			}
			assertProfileRun(t, rec.Body.Bytes(), run, started)
			if api.calls != 1 || api.profileID != "andrew" || api.key.String() != key {
				t.Fatalf("StartRun calls %d profile %q key %s", api.calls, api.profileID, api.key)
			}
			if !tt.replay {
				return
			}
			again := postProfileRunReq(h, tt.key, false, "")
			if again.Code != http.StatusAccepted {
				t.Fatalf("replay status %d %s", again.Code, again.Body.String())
			}
			assertProfileRun(t, again.Body.Bytes(), run, started)
			if api.calls != 2 {
				t.Fatalf("replay calls %d", api.calls)
			}
		})
	}
}

func assertProfileRun(t *testing.T, raw []byte, run scoringschema.Run, started time.Time) {
	t.Helper()
	var got schema.ProfileRunResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.RunID != run.ID || got.ProfileID != run.ProfileID || got.Status != scoringschema.RunStatusRunning.String() {
		t.Fatalf("body %#v", got)
	}
	if !got.StartedAt.Equal(started) || got.FinishedAt != nil || got.JobsScored != 0 || got.SourcesSkipped != 0 {
		t.Fatalf("body %#v", got)
	}
}

func postProfileRunReq(h *HTTPHandler, key string, omitKey bool, body string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader == nil {
		req = httptest.NewRequest(http.MethodPost, "/api/v1/profiles/andrew/runs", nil)
	} else {
		req = httptest.NewRequest(http.MethodPost, "/api/v1/profiles/andrew/runs", reader)
	}
	if !omitKey {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type profileRunScoring struct {
	run       scoringschema.Run
	err       error
	calls     int
	profileID string
	key       uuid.UUID
}

func (f *profileRunScoring) StartRun(_ context.Context, profileID string, key uuid.UUID) (scoringschema.Run, error) {
	f.calls++
	f.profileID = profileID
	f.key = key
	if f.err != nil {
		return scoringschema.Run{}, f.err
	}
	out := f.run
	out.IdempotencyKey = key
	return out, nil
}

func (f *profileRunScoring) LatestRun(context.Context, string) (scoringschema.Run, error) {
	return scoringschema.Run{}, nil
}

func (f *profileRunScoring) ListJobs(context.Context, scoringschema.ListJobsParams) (scoringschema.JobPage, error) {
	return scoringschema.JobPage{}, nil
}

func (f *profileRunScoring) SetUserStatus(context.Context, string, string, scoringschema.UserStatus) (scoringschema.ListedJob, error) {
	return scoringschema.ListedJob{}, nil
}
