package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
)

func TestWriteAPIError_internalServerErrorSanitized(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteAPIError(rec, http.StatusInternalServerError, "db_down", "secret dsn postgres://…")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d", rec.Code)
	}
	var body schema.APIErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "db_down" || body.Error.Message != "internal server error" {
		t.Fatalf("body: %+v", body)
	}
}

func TestWriteSlotLimitReached_shape(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteSlotLimitReached(rec, "too many")
	var body schema.SlotLimitReachedBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "slot_limit_reached" || body.Limit != 3 {
		t.Fatalf("body: %+v", body)
	}
}

func TestReadJSON_invalidSyntax(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	var dst struct{}
	if ReadJSON(rec, req, &dst) {
		t.Fatal("expected false")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestReadJSON_trailingGarbage(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{} []`))
	req.Header.Set("Content-Type", "application/json")
	var dst struct{}
	if ReadJSON(rec, req, &dst) {
		t.Fatal("expected false")
	}
}
