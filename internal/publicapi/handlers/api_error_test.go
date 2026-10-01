package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/rs/zerolog"
)

func TestGetSlots_unmappedErrorIsInternal(t *testing.T) {
	const raw = "postgres dial failed secret-token-9f3a"
	h := NewHTTPHandler(nil, Deps{
		Logger:  zerolog.Nop(),
		Slots:   &mockSlots{listErr: errors.New(raw)},
		Profile: stubProfile{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), raw) {
		t.Fatalf("body leaked raw error: %s", rec.Body.String())
	}
	var body schema.APIErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != schema.APIErrorCodeUnexpected.String() || body.Error.Message != "Internal server error." {
		t.Fatalf("error: got %+v", body.Error)
	}
}

func TestWrongMethodIsRegistry405(t *testing.T) {
	h := NewHTTPHandler(nil, Deps{
		Logger:  zerolog.Nop(),
		Slots:   &mockSlots{},
		Profile: stubProfile{},
	})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/slots", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type %q", rec.Header().Get("Content-Type"))
	}
	var body schema.APIErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	assertAPIError(t, body.Error, schema.APIErrorCodeMethodNotAllowed)

	miss := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	missRec := httptest.NewRecorder()
	h.ServeHTTP(missRec, miss)
	if missRec.Code != http.StatusNotFound {
		t.Fatalf("missing status %d %s", missRec.Code, missRec.Body.String())
	}
	if strings.Contains(missRec.Body.String(), schema.APIErrorCodeMethodNotAllowed.String()) {
		t.Fatalf("404 rewritten: %s", missRec.Body.String())
	}
}

func TestAPIErrorRegistryJSONCodeSet(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "schema", "generated", "api-error-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(rows))
	for i, row := range rows {
		got[i] = row.Code
	}
	want := schema.Codes()
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("registry codes %v constants %v", got, want)
	}
}
