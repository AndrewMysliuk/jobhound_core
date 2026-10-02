package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
	"github.com/rs/zerolog"
)

type mockSlotsStage2HTTP struct {
	mockSlotsJobs
	lastListParams slotschema.ListJobsParams
}

func (m *mockSlotsStage2HTTP) ListJobs(ctx context.Context, p slotschema.ListJobsParams) (schema.JobListResponse, error) {
	m.lastListParams = p
	return m.mockSlotsJobs.ListJobs(ctx, p)
}

func postStage2(h *HTTPHandler, sid, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/slots/"+sid+"/stages/2/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPostStage2Run_rulesBody(t *testing.T) {
	sid := "11111111-1111-4111-8111-111111111111"
	ms := &mockSlotsStageRuns{
		run2Ret: &schema.StageRunAcceptedResponse{SlotID: sid, Stage: 2},
	}
	h := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: ms, Profile: stubProfile{}})

	t.Run("valid_rules_202", func(t *testing.T) {
		rec := postStage2(h, sid, `{"rules":[{"id":"java_title","field":"title","op":"phrase","values":["java"],"action":"reject"}]}`)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("listing_phrase_202", func(t *testing.T) {
		rec := postStage2(h, sid, `{"rules":[{"id":"java_listing","field":"listing","op":"phrase","values":["java"],"action":"reject"}]}`)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("include_exclude_400", func(t *testing.T) {
		rec := postStage2(h, sid, `{"include":["x"],"exclude":["y"]}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	cases := []struct {
		name string
		body string
	}{
		{"duplicate_id", `{"rules":[{"id":"r","field":"title","op":"phrase","values":["a"],"action":"reject"},{"id":"r","field":"body","op":"phrase","values":["b"],"action":"flag"}]}`},
		{"weight_on_reject", `{"rules":[{"id":"r","field":"title","op":"phrase","values":["a"],"action":"reject","weight":1}]}`},
		{"weight_on_flag", `{"rules":[{"id":"r","field":"title","op":"phrase","values":["a"],"action":"flag","weight":1}]}`},
		{"boost_missing_weight", `{"rules":[{"id":"r","field":"position","op":"any","values":["frontend"],"action":"boost"}]}`},
		{"boost_weight_zero", `{"rules":[{"id":"r","field":"position","op":"any","values":["frontend"],"action":"boost","weight":0}]}`},
		{"penalty_missing_weight", `{"rules":[{"id":"r","field":"position","op":"any","values":["frontend"],"action":"penalty"}]}`},
		{"penalty_weight_positive", `{"rules":[{"id":"r","field":"position","op":"any","values":["frontend"],"action":"penalty","weight":1}]}`},
		{"phrase_on_position_field", `{"rules":[{"id":"r","field":"position","op":"phrase","values":["java"],"action":"reject"}]}`},
		{"listing_op_any", `{"rules":[{"id":"r","field":"listing","op":"any","values":["java"],"action":"reject"}]}`},
		{"negation_on_any", `{"rules":[{"id":"r","field":"position","op":"any","values":["frontend"],"action":"reject","negation_window":2}]}`},
		{"unknown_position", `{"rules":[{"id":"r","field":"position","op":"any","values":["not-a-position"],"action":"reject"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postStage2(h, sid, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d %s", rec.Code, rec.Body.String())
			}
			if tc.name == "unknown_position" || tc.name == "listing_op_any" {
				var errBody schema.APIErrorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
					t.Fatal(err)
				}
				assertAPIError(t, errBody.Error, schema.APIErrorCodeValidationFailed)
			}
		})
	}
}

func TestGetStageJobs_stage2QueryAndDebug(t *testing.T) {
	sid := "11111111-1111-4111-8111-111111111111"
	t0 := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	st := "PASSED_STAGE_2"
	baseItem := schema.JobListItem{
		JobID: "j1", Title: "t", Company: "c", Description: "desc", SourceID: "src",
		URL: "https://list", ApplyURL: "u", FirstSeenAt: t0, PostedAt: &t0, Status: &st,
		Stage3Rationale: nil,
	}
	ms := &mockSlotsStage2HTTP{
		mockSlotsJobs: mockSlotsJobs{
			mockSlots: mockSlots{},
			listResp: schema.JobListResponse{
				Items: []schema.JobListItem{baseItem},
				Page:  1, Limit: 50, Total: 1,
			},
		},
	}
	h := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: ms, Profile: stubProfile{}})

	t.Run("unknown_status_400", func(t *testing.T) {
		ms404 := &mockSlotsJobs{listErr: slots.ErrInvalidJobListQuery}
		hB := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: ms404, Profile: stubProfile{}})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/2/jobs?status=BOGUS", nil)
		rec := httptest.NewRecorder()
		hB.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("unknown_stage_2_accepted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/2/jobs?status=UNKNOWN_STAGE_2", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if ms.lastListParams.StatusQuery != "UNKNOWN_STAGE_2" {
			t.Fatalf("status query %q", ms.lastListParams.StatusQuery)
		}
	})

	t.Run("invalid_debug_400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/2/jobs?debug=yes", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
		var errBody schema.APIErrorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
			t.Fatal(err)
		}
		assertAPIError(t, errBody.Error, schema.APIErrorCodeInvalidQuery)
	})

	t.Run("without_debug_byte_baseline", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/2/jobs", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		baseline := rec.Body.Bytes()
		if ms.lastListParams.Stage2Debug {
			t.Fatal("expected Stage2Debug false")
		}

		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/2/jobs?debug=0", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusBadRequest {
			t.Fatalf("debug=0 status %d", rec2.Code)
		}

		req3 := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/2/jobs", nil)
		rec3 := httptest.NewRecorder()
		h.ServeHTTP(rec3, req3)
		if !bytes.Equal(baseline, rec3.Body.Bytes()) {
			t.Fatalf("responses differ without debug")
		}
	})

	t.Run("debug_adds_hits_and_boost", func(t *testing.T) {
		hits := []pipeline.Stage2Hit{{RuleID: "r1", Action: "boost", Matched: "go"}}
		boost := 5
		dbgItem := baseItem
		dbgItem.Hits = &hits
		dbgItem.Stage2Boost = &boost
		msDbg := &mockSlotsStage2HTTP{
			mockSlotsJobs: mockSlotsJobs{
				mockSlots: mockSlots{},
				listResp: schema.JobListResponse{
					Items: []schema.JobListItem{dbgItem},
					Page:  1, Limit: 50, Total: 1,
				},
			},
		}
		hDbg := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: msDbg, Profile: stubProfile{}})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slots/"+sid+"/stages/2/jobs?debug=1", nil)
		rec := httptest.NewRecorder()
		hDbg.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		if !msDbg.lastListParams.Stage2Debug {
			t.Fatal("expected Stage2Debug true")
		}
		var body schema.JobListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Items) != 1 || body.Items[0].Hits == nil || body.Items[0].Stage2Boost == nil {
			t.Fatalf("got %+v", body.Items[0])
		}
		if len(*body.Items[0].Hits) != 1 || *body.Items[0].Stage2Boost != 5 {
			t.Fatalf("hits/boost %+v %v", body.Items[0].Hits, body.Items[0].Stage2Boost)
		}
	})
}
