package handlers

import (
	"encoding/json"
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

func TestWrongMethodIsRegistry405(t *testing.T) {
	h := NewHTTPHandler(nil, Deps{
		Logger: zerolog.Nop(),
	})
	health := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	healthRec := httptest.NewRecorder()
	h.ServeHTTP(healthRec, health)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status %d %s", healthRec.Code, healthRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
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

	slots := httptest.NewRequest(http.MethodGet, "/api/v1/slots", nil)
	slotsRec := httptest.NewRecorder()
	h.ServeHTTP(slotsRec, slots)
	if slotsRec.Code != http.StatusNotFound {
		t.Fatalf("slots status %d %s", slotsRec.Code, slotsRec.Body.String())
	}
}

func assertAPIError(t *testing.T, got schema.APIErrorDetail, code schema.APIErrorCode) {
	t.Helper()
	spec, ok := schema.Lookup(code)
	if !ok {
		t.Fatalf("code %s is not registered", code)
	}
	if got.Code != spec.Code.String() || got.Message != spec.Message {
		t.Fatalf("error: got %+v want %s %q", got, spec.Code, spec.Message)
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
