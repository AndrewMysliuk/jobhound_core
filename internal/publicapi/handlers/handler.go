// Package handlers is the product HTTP API for the browser client (not collectors debug HTTP).
package handlers

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/profile"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

// Deps are shared dependencies for route handlers (wired from cmd/api).
type Deps struct {
	Slots   slots.API
	Profile profile.API
}

// HTTPHandler serves /api/v1 routes behind CORS middleware.
type HTTPHandler struct {
	mux   *http.ServeMux
	chain http.Handler
	deps  Deps
}

// NewHTTPHandler registers routes and wraps the mux with CORS for configured origins.
func NewHTTPHandler(corsAllowedOrigins []string, deps Deps) *HTTPHandler {
	h := &HTTPHandler{
		mux:  http.NewServeMux(),
		deps: deps,
	}
	h.registerRoutes()
	h.chain = withCORS(corsAllowedOrigins, h.mux)
	return h
}

func (h *HTTPHandler) registerRoutes() {
	h.mux.HandleFunc("GET /api/v1/health", h.getHealth)
	h.mux.HandleFunc("GET /api/v1/profile", h.getProfile)
	h.mux.HandleFunc("PUT /api/v1/profile", h.putProfile)
	h.mux.HandleFunc("GET /api/v1/slots", h.getSlots)
	h.mux.HandleFunc("POST /api/v1/slots", h.postSlots)
	h.mux.HandleFunc("GET /api/v1/slots/{slot_id}", h.getSlot)
	h.mux.HandleFunc("DELETE /api/v1/slots/{slot_id}", h.deleteSlot)
	h.mux.HandleFunc("POST /api/v1/slots/{slot_id}/stages/2/run", h.postStage2Run)
	h.mux.HandleFunc("POST /api/v1/slots/{slot_id}/stages/3/run", h.postStage3Run)
	h.mux.HandleFunc("GET /api/v1/slots/{slot_id}/stages/{stage}/jobs", h.getStageJobs)
	h.mux.HandleFunc("PATCH /api/v1/slots/{slot_id}/stages/{stage}/jobs/{job_id}", h.patchStageJobBucket)
}

// ServeHTTP applies CORS then dispatches to registered routes.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.chain.ServeHTTP(w, r)
}
