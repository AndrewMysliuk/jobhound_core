// Package debughttp is development-only HTTP to exercise collectors (not the product API).
package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/dou"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/rs/zerolog"
)

// PathEuropeRemotely is the registered method+path for the Europe Remotely debug fetch.
const PathEuropeRemotely = "POST /debug/collectors/europe_remotely"

// PathWorkingNomads is the registered method+path for the Working Nomads debug fetch.
const PathWorkingNomads = "POST /debug/collectors/working_nomads"

// PathDouUA is the registered method+path for the DOU.ua debug fetch.
const PathDouUA = "POST /debug/collectors/dou_ua"

// PathHimalayas is the registered method+path for the Himalayas debug fetch.
const PathHimalayas = "POST /debug/collectors/himalayas"

// HTTPHandler wires debug routes on a ServeMux (omg-bo style: handler.go + registerRoutes + one file per route).
type HTTPHandler struct {
	mux   *http.ServeMux
	chain http.Handler
	log   zerolog.Logger

	europeRemotely         collectors.Collector
	workingNomads          collectors.Collector
	douUa                  collectors.Collector
	himalayas              collectors.Collector
	workingNomadsConcrete  *workingnomads.WorkingNomads
	europeRemotelyConcrete *europeremotely.EuropeRemotely
	douUaConcrete          *dou.DOU
	himalayasConcrete      *himalayas.Himalayas
}

// NewHTTPHandler returns a handler with GET /health and POST /debug/collectors/* registered.
// europeRemotely, workingNomads, and douUa must not be nil; himalayas may be nil (route returns 500).
// Concrete types may be nil (tests); when set, POST bodies can override per-source settings without mutating bootstrap.
// log is the binary root logger (e.g. from logging.NewRoot); use zerolog.Nop() in tests.
func NewHTTPHandler(
	europeRemotely, workingNomads, douUa, himalayasColl collectors.Collector,
	workingNomadsConcrete *workingnomads.WorkingNomads,
	europeRemotelyConcrete *europeremotely.EuropeRemotely,
	douUaConcrete *dou.DOU,
	himalayasConcrete *himalayas.Himalayas,
	log zerolog.Logger,
) *HTTPHandler {
	h := &HTTPHandler{
		mux:                    http.NewServeMux(),
		log:                    log,
		europeRemotely:         europeRemotely,
		workingNomads:          workingNomads,
		douUa:                  douUa,
		himalayas:              himalayasColl,
		workingNomadsConcrete:  workingNomadsConcrete,
		europeRemotelyConcrete: europeRemotelyConcrete,
		douUaConcrete:          douUaConcrete,
		himalayasConcrete:      himalayasConcrete,
	}
	h.registerRoutes()
	h.chain = logging.RequestIDMiddleware(h.mux)
	return h
}

func (h *HTTPHandler) registerRoutes() {
	h.mux.HandleFunc("GET /health", h.health)
	h.mux.HandleFunc(PathEuropeRemotely, h.postEuropeRemotely)
	h.mux.HandleFunc(PathWorkingNomads, h.postWorkingNomads)
	h.mux.HandleFunc(PathDouUA, h.postDouUA)
	h.mux.HandleFunc(PathHimalayas, h.postHimalayas)
}

// ServeHTTP applies request-id middleware then dispatches to registered routes.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.chain.ServeHTTP(w, r)
}
