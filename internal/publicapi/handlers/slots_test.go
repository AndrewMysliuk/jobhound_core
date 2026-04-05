package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	"github.com/rs/zerolog"
)

type mockSlots struct {
	listResp  schema.SlotsListResponse
	listErr   error
	createRet *schema.SlotCard
	createErr error
	getRet    *schema.SlotCard
	getErr    error
	delErr    error
}

func (m *mockSlots) List(ctx context.Context) (schema.SlotsListResponse, error) {
	return m.listResp, m.listErr
}

func (m *mockSlots) Create(ctx context.Context, name string) (*schema.SlotCard, error) {
	return m.createRet, m.createErr
}

func (m *mockSlots) Get(ctx context.Context, slotID string) (*schema.SlotCard, error) {
	return m.getRet, m.getErr
}

func (m *mockSlots) Delete(ctx context.Context, slotID string) error {
	return m.delErr
}

func (m *mockSlots) RunStage2(context.Context, string, []string, []string) (*schema.StageRunAcceptedResponse, error) {
	return nil, errors.New("unexpected RunStage2 in slots_test mock")
}

func (m *mockSlots) RunStage3(context.Context, string, int) (*schema.StageRunAcceptedResponse, error) {
	return nil, errors.New("unexpected RunStage3 in slots_test mock")
}

func (m *mockSlots) ListJobs(context.Context, string, int, int, int, string) (schema.JobListResponse, error) {
	return schema.JobListResponse{}, errors.New("unexpected ListJobs in slots_test mock")
}

func (m *mockSlots) PatchJobBucket(context.Context, string, int, string, schema.JobBucket) (*schema.PatchJobBucketResponse, error) {
	return nil, errors.New("unexpected PatchJobBucket in slots_test mock")
}

type stubProfile struct{}

func (stubProfile) Get(context.Context) (schema.ProfileResponse, error) {
	return schema.ProfileResponse{Text: "", UpdatedAt: time.Unix(0, 0).UTC()}, nil
}

func (stubProfile) Put(context.Context, string) (schema.ProfileResponse, error) {
	return schema.ProfileResponse{}, errors.New("unexpected Put in slots_test stubProfile")
}

func TestSlotsRoutes_tableDriven(t *testing.T) {
	t0 := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	sampleCard := &schema.SlotCard{
		ID:        "11111111-1111-4111-8111-111111111111",
		Name:      "n",
		CreatedAt: t0,
		Stage1: schema.StageFull{
			State:     schema.StageStateRunning,
			StartedAt: &t0,
		},
		Stage2: schema.StageFull{State: schema.StageStateIdle},
		Stage3: schema.StageFull{State: schema.StageStateIdle},
	}

	tests := []struct {
		name      string
		mock      *mockSlots
		method    string
		path      string
		body      string
		wantCode  int
		checkBody func(t *testing.T, body []byte)
		skipBody  bool
	}{
		{
			name: "get_list_ok",
			mock: &mockSlots{
				listResp: schema.SlotsListResponse{
					Slots: []schema.SlotListItem{
						{
							ID:        sampleCard.ID,
							Name:      sampleCard.Name,
							CreatedAt: sampleCard.CreatedAt,
							Stage1:    schema.StageCompact{State: sampleCard.Stage1.State},
							Stage2:    schema.StageCompact{State: schema.StageStateIdle},
							Stage3:    schema.StageCompact{State: schema.StageStateIdle},
						},
					},
				},
			},
			method:   http.MethodGet,
			path:     "/api/v1/slots",
			wantCode: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var got schema.SlotsListResponse
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				if len(got.Slots) != 1 || got.Slots[0].ID != sampleCard.ID {
					t.Fatalf("unexpected list: %+v", got)
				}
			},
		},
		{
			name:   "post_create_ok",
			mock:   &mockSlots{createRet: sampleCard},
			method: http.MethodPost, path: "/api/v1/slots",
			body:     `{"name":"n"}`,
			wantCode: http.StatusCreated,
			checkBody: func(t *testing.T, body []byte) {
				var got schema.SlotCard
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				if got.ID != sampleCard.ID || got.Name != sampleCard.Name {
					t.Fatalf("card: %+v", got)
				}
			},
		},
		{
			name:   "post_slot_limit",
			mock:   &mockSlots{createErr: slots.ErrSlotLimitReached},
			method: http.MethodPost, path: "/api/v1/slots",
			body:     `{"name":"x"}`,
			wantCode: http.StatusConflict,
			checkBody: func(t *testing.T, body []byte) {
				var got schema.SlotLimitReachedBody
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				if got.Error.Code != "slot_limit_reached" || got.Limit != 3 {
					t.Fatalf("body: %+v", got)
				}
			},
		},
		{
			name:     "get_slot_ok",
			mock:     &mockSlots{getRet: sampleCard},
			method:   http.MethodGet,
			path:     "/api/v1/slots/11111111-1111-4111-8111-111111111111",
			wantCode: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var got schema.SlotCard
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				if got.ID != sampleCard.ID {
					t.Fatalf("id %q", got.ID)
				}
			},
		},
		{
			name:   "get_slot_404",
			mock:   &mockSlots{getErr: slots.ErrNotFound},
			method: http.MethodGet, path: "/api/v1/slots/22222222-2222-4222-8222-222222222222",
			wantCode: http.StatusNotFound,
			checkBody: func(t *testing.T, body []byte) {
				var got schema.APIErrorBody
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				if got.Error.Code != "not_found" {
					t.Fatalf("code %q", got.Error.Code)
				}
			},
		},
		{
			name:     "delete_slot_204",
			mock:     &mockSlots{},
			method:   http.MethodDelete,
			path:     "/api/v1/slots/33333333-3333-4333-8333-333333333333",
			wantCode: http.StatusNoContent,
			skipBody: true,
		},
		{
			name:     "delete_slot_404",
			mock:     &mockSlots{delErr: slots.ErrNotFound},
			method:   http.MethodDelete,
			path:     "/api/v1/slots/44444444-4444-4444-8444-444444444444",
			wantCode: http.StatusNotFound,
			checkBody: func(t *testing.T, body []byte) {
				var got schema.APIErrorBody
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				if got.Error.Code != "not_found" {
					t.Fatalf("code %q", got.Error.Code)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHTTPHandler(nil, Deps{Logger: zerolog.Nop(), Slots: tt.mock, Profile: stubProfile{}})
			var reqBody *bytes.Reader
			if tt.body != "" {
				reqBody = bytes.NewReader([]byte(tt.body))
			} else {
				reqBody = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tt.method, tt.path, reqBody)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Fatalf("status: got %d want %d body=%s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if !tt.skipBody && tt.checkBody != nil {
				tt.checkBody(t, rec.Body.Bytes())
			}
		})
	}
}
