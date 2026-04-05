package logging

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware_generatesAndEchoes(t *testing.T) {
	var got string
	h := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
			got = v
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got == "" {
		t.Fatal("expected request_id on context")
	}
	if rec.Header().Get(HeaderRequestID) != got {
		t.Fatalf("response header: got %q want %q", rec.Header().Get(HeaderRequestID), got)
	}
}

func TestRequestIDMiddleware_respectsClientHeader(t *testing.T) {
	const want = "client-correlation-1"
	var got string
	h := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = r.Context().Value(ctxKeyRequestID).(string)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, want)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got != want {
		t.Fatalf("context id: got %q want %q", got, want)
	}
	if rec.Header().Get(HeaderRequestID) != want {
		t.Fatalf("response: got %q", rec.Header().Get(HeaderRequestID))
	}
}
