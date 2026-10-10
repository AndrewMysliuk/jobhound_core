// Package handlers is the product HTTP API for the browser client (not collectors debug HTTP).
package handlers

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
	"github.com/rs/zerolog"
)

// Deps are shared dependencies for route handlers (wired from cmd/api).
type Deps struct {
	Logger   zerolog.Logger
	Profiles profiles.Store
	Scoring  scoring.API
}

// HTTPHandler serves /api/v1 routes behind CORS middleware.
type HTTPHandler struct {
	mux   *http.ServeMux
	chain http.Handler
	deps  Deps
}

// NewHTTPHandler registers routes and wraps the mux: request id (outer) then CORS, then routes.
func NewHTTPHandler(corsAllowedOrigins []string, deps Deps) *HTTPHandler {
	h := &HTTPHandler{
		mux:  http.NewServeMux(),
		deps: deps,
	}
	h.registerRoutes()
	h.chain = logging.RequestIDMiddleware(apputils.WithCORS(corsAllowedOrigins, apiMux{mux: h.mux}))
	return h
}

func (h *HTTPHandler) routeLog(r *http.Request, handlerName string) zerolog.Logger {
	return logging.EnrichWithContext(r.Context(), h.deps.Logger.With().Str(logging.FieldHandler, handlerName).Logger())
}

func (h *HTTPHandler) registerRoutes() {
	h.mux.HandleFunc("GET /api/v1/health", h.getHealth)
	h.mux.HandleFunc("GET /api/v1/profiles", h.getProfiles)
	h.mux.HandleFunc("PATCH /api/v1/profiles/{profile_id}/jobs/{job_id}", h.patchProfileJob)
	h.mux.HandleFunc("GET /api/v1/profiles/{profile_id}/runs/latest", h.getProfileRunLatest)
	h.mux.HandleFunc("GET /api/v1/profiles/{profile_id}/jobs", h.getProfileJobs)
	h.mux.HandleFunc("POST /api/v1/profiles/{profile_id}/runs", h.postProfileRun)
}

// ServeHTTP applies request-id and CORS then dispatches to registered routes.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.chain.ServeHTTP(w, r)
}

// apiMux replaces ServeMux's plain-text 405. Method-specific patterns never
// call the route handler, so the registry write has to happen here.
type apiMux struct {
	mux *http.ServeMux
}

func (m apiMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	guard := &statusGuard{ResponseWriter: w}
	m.mux.ServeHTTP(guard, r)
	if guard.status == http.StatusMethodNotAllowed {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeMethodNotAllowed})
	}
}

// statusGuard drops a 405 body so WriteError can write the registry envelope.
// Other statuses pass through.
type statusGuard struct {
	http.ResponseWriter
	status int
}

func (g *statusGuard) WriteHeader(code int) {
	if g.status != 0 {
		return
	}
	g.status = code
	if code == http.StatusMethodNotAllowed {
		return
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *statusGuard) Write(p []byte) (int, error) {
	if g.status == http.StatusMethodNotAllowed {
		return len(p), nil
	}
	if g.status == 0 {
		g.status = http.StatusOK
	}
	return g.ResponseWriter.Write(p)
}

func (g *statusGuard) Unwrap() http.ResponseWriter { return g.ResponseWriter }
