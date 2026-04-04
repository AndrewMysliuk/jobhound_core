package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

type mockProfile struct {
	getRet schema.ProfileResponse
	getErr error
	putRet schema.ProfileResponse
	putErr error
}

func (m *mockProfile) Get(ctx context.Context) (schema.ProfileResponse, error) {
	return m.getRet, m.getErr
}

func (m *mockProfile) Put(ctx context.Context, text string) (schema.ProfileResponse, error) {
	return m.putRet, m.putErr
}

type mockSlotsStageRuns struct {
	mockSlots
	run2Ret *schema.StageRunAcceptedResponse
	run2Err error
	run3Ret *schema.StageRunAcceptedResponse
	run3Err error
}

func (m *mockSlotsStageRuns) RunStage2(ctx context.Context, slotID string, include, exclude []string) (*schema.StageRunAcceptedResponse, error) {
	return m.run2Ret, m.run2Err
}

func (m *mockSlotsStageRuns) RunStage3(ctx context.Context, slotID string, maxJobs int) (*schema.StageRunAcceptedResponse, error) {
	return m.run3Ret, m.run3Err
}

func TestProfileRoutes_roundTrip(t *testing.T) {
	t0 := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	prof := &mockProfile{
		getRet: schema.ProfileResponse{Text: "hello", UpdatedAt: t0},
		putRet: schema.ProfileResponse{Text: "hello", UpdatedAt: t0},
	}
	h := NewHTTPHandler(nil, Deps{Slots: &mockSlots{}, Profile: prof})

	t.Run("get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		var got schema.ProfileResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Text != "hello" {
			t.Fatalf("text %q", got.Text)
		}
	})

	t.Run("put_invalid_json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/profile", bytes.NewBufferString(`{`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("put_ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/profile", bytes.NewBufferString(`{"text":"hello"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
	})
}

func TestPostStage2Run_validationAnd409(t *testing.T) {
	sid := "11111111-1111-4111-8111-111111111111"
	ms := &mockSlotsStageRuns{
		run2Ret: &schema.StageRunAcceptedResponse{SlotID: sid, Stage: 2},
	}
	h := NewHTTPHandler(nil, Deps{Slots: ms, Profile: stubProfile{}})

	t.Run("missing_include", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/slots/"+sid+"/stages/2/run", bytes.NewBufferString(`{"exclude":[]}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/slots/"+sid+"/stages/2/run", bytes.NewBufferString(`{"include":["a"],"exclude":["b"]}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
	})

	ms409 := &mockSlotsStageRuns{run2Err: slots.ErrStageAlreadyRunning}
	h409 := NewHTTPHandler(nil, Deps{Slots: ms409, Profile: stubProfile{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/slots/"+sid+"/stages/2/run", bytes.NewBufferString(`{"include":[],"exclude":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h409.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
	var body schema.APIErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "stage_already_running" {
		t.Fatalf("code %q", body.Error.Code)
	}
}

func TestPostStage3Run_404_and_422(t *testing.T) {
	sid := "22222222-2222-4222-8222-222222222222"
	ms := &mockSlotsStageRuns{run3Err: slots.ErrNotFound}
	h := NewHTTPHandler(nil, Deps{Slots: ms, Profile: stubProfile{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/slots/"+sid+"/stages/3/run", bytes.NewBufferString(`{"max_jobs":5}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}

	ms2 := &mockSlotsStageRuns{run3Err: slots.ErrNoPipelineRun}
	h2 := NewHTTPHandler(nil, Deps{Slots: ms2, Profile: stubProfile{}})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/slots/"+sid+"/stages/3/run", bytes.NewBufferString(`{"max_jobs":5}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", rec2.Code)
	}

	ms3 := &mockSlotsStageRuns{run3Err: slots.ErrProfileRequired}
	h3 := NewHTTPHandler(nil, Deps{Slots: ms3, Profile: stubProfile{}})
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/slots/"+sid+"/stages/3/run", bytes.NewBufferString(`{"max_jobs":5}`))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	h3.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", rec3.Code)
	}
}

func TestPostStage3Run_maxJobsValidation(t *testing.T) {
	sid := "33333333-3333-4333-8333-333333333333"
	ms := &mockSlotsStageRuns{}
	h := NewHTTPHandler(nil, Deps{Slots: ms, Profile: stubProfile{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/slots/"+sid+"/stages/3/run", bytes.NewBufferString(`{"max_jobs":0}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}
