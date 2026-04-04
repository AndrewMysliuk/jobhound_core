package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCORS_preflightAllowedOrigin(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withCORS([]string{"http://localhost:8080"}, mux)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8080" {
		t.Fatalf("ACAO: got %q", got)
	}
}

func TestWithCORS_disallowedOriginNoACAO(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := withCORS([]string{"http://allowed.example"}, mux)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no ACAO, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
