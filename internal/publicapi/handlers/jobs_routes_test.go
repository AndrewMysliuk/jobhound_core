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
	"github.com/rs/zerolog"
)

type mockSlotsJobs struct {
	mockSlots
	listResp schema.JobListResponse
	listErr  error
	patchRet *schema.PatchJobBucketResponse
	patchErr error
}

func (m *mockSlotsJobs) ListJobs(ctx context.Context, slotID string, stage, page, limit int, statusQuery string) (schema.JobListResponse, error) {
	if m.listErr != nil {
		return schema.JobListResponse{}, m.listErr
	}
	_ = ctx
	_ = slotID
	_ = stage
	_ = statusQuery
	out := m.listResp
	out.Page = page
	out.Limit = limit
	return out, nil
}

func (m *mockSlotsJobs) PatchJobBucket(ctx context.Context, slotID string, stage int, jobID string, bucket schema.JobBucket) (*schema.PatchJobBucketResponse, error) {
	_ = ctx
	_ = slotID
	_ = stage
	_ = jobID
	_ = bucket
	return m.patchRet, m.patchErr
}

func TestGetStageJobs_queryAnd404(t *testing.T) {
	sid := "11111111-1111-4111-8111-111111111111"
	t0 := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ms := &mockSlotsJobs{
		listResp: schema.JobListResponse{
			Items: []schema.JobListItem{
				{
					JobID: "j1", Title: "t", Company: "c", SourceID: "src", ApplyURL: "u",
					FirstSeenAt: t0, PostedAt: &t0, Stage3Rationale: nil,
				},
			},
			Page:  2,
			Limit: 10,
			Total: 25,
		},
	}
	h := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: ms, Profile: stubProfile{}})

	t.Run("limit_max_ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/1/jobs?page=1&limit=100", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("limit_over_max", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/1/jobs?limit=101", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("stage1_status_filter_rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/1/jobs?status=PASSED_STAGE_2", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("empty_list", func(t *testing.T) {
		msEmpty := &mockSlotsJobs{listResp: schema.JobListResponse{Items: []schema.JobListItem{}, Page: 1, Limit: 50, Total: 0}}
		hE := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: msEmpty, Profile: stubProfile{}})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/2/jobs", nil)
		rec := httptest.NewRecorder()
		hE.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		var body schema.JobListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Items) != 0 || body.Total != 0 {
			t.Fatalf("got %+v", body)
		}
	})

	t.Run("404_slot", func(t *testing.T) {
		ms404 := &mockSlotsJobs{listErr: slots.ErrNotFound}
		h404 := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: ms404, Profile: stubProfile{}})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/1/jobs", nil)
		rec := httptest.NewRecorder()
		h404.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("invalid_status", func(t *testing.T) {
		msB := &mockSlotsJobs{listErr: slots.ErrInvalidJobListQuery}
		hB := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: msB, Profile: stubProfile{}})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/2/jobs?status=BOGUS", nil)
		rec := httptest.NewRecorder()
		hB.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})
}

func TestPatchStageJobBucket(t *testing.T) {
	sid := "22222222-2222-4222-8222-222222222222"
	ms := &mockSlotsJobs{
		patchRet: &schema.PatchJobBucketResponse{JobID: "jx", Bucket: schema.JobBucketPassed},
	}
	h := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: ms, Profile: stubProfile{}})

	t.Run("ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/slots/"+sid+"/stages/2/jobs/jx", bytes.NewBufferString(`{"bucket":"passed"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("stage1_rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/slots/"+sid+"/stages/1/jobs/jx", bytes.NewBufferString(`{"bucket":"passed"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("404", func(t *testing.T) {
		ms404 := &mockSlotsJobs{patchErr: slots.ErrNotFound}
		h404 := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: ms404, Profile: stubProfile{}})
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/slots/"+sid+"/stages/3/jobs/missing", bytes.NewBufferString(`{"bucket":"failed"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h404.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d", rec.Code)
		}
	})
}
