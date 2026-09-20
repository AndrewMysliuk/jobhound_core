// Package debughttp is development-only HTTP to exercise collectors (not the product API).
package debughttp

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/rs/zerolog"
)

// PathEuropeRemotely is the registered method+path for the Europe Remotely debug fetch.
const PathEuropeRemotely = "POST /debug/collectors/europe_remotely"

// PathWorkingNomads is the registered method+path for the Working Nomads debug fetch.
const PathWorkingNomads = "POST /debug/collectors/working_nomads"

// PathHimalayas is the registered method+path for the Himalayas debug fetch.
const PathHimalayas = "POST /debug/collectors/himalayas"

// PathBuiltin is the registered method+path for the Built In debug fetch.
const PathBuiltin = "POST /debug/collectors/builtin"

// PathRemotifyEurope is the registered method+path for the Remotify Europe debug fetch.
const PathRemotifyEurope = "POST /debug/collectors/remotify_europe"

// PathWeWorkRemotely is the registered method+path for the We Work Remotely debug fetch.
const PathWeWorkRemotely = "POST /debug/collectors/we_work_remotely"

// PathWellfound is the registered method+path for the Wellfound debug fetch.
const PathWellfound = "POST /debug/collectors/wellfound"

// PathVueJobs is the registered method+path for the VueJobs debug fetch.
const PathVueJobs = "POST /debug/collectors/vue_jobs"

// PathGolangCafe is the registered method+path for the Golang Cafe debug fetch.
const PathGolangCafe = "POST /debug/collectors/golang_cafe"

// HTTPHandler wires debug routes on a ServeMux (omg-bo style: handler.go + registerRoutes + one file per route).
type HTTPHandler struct {
	mux   *http.ServeMux
	chain http.Handler
	log   zerolog.Logger

	europeRemotely         collectors.Collector
	workingNomads          collectors.Collector
	himalayas              collectors.Collector
	builtin                collectors.Collector
	remotifyEurope         collectors.Collector
	weWorkRemotely         collectors.Collector
	wellfound              collectors.Collector
	vueJobs                collectors.Collector
	golangCafe             collectors.Collector
	workingNomadsConcrete  *workingnomads.WorkingNomads
	europeRemotelyConcrete *europeremotely.EuropeRemotely
	himalayasConcrete      *himalayas.Himalayas
	builtinConcrete        *builtin.BuiltIn
	remotifyEuropeConcrete *remotifyeurope.RemotifyEurope
	wellfoundConcrete      *wellfound.Wellfound
	vueJobsConcrete        *vuejobs.VueJobs
	golangCafeConcrete     *golangcafe.GolangCafe
}

// NewHTTPHandler returns a handler with GET /health and POST /debug/collectors/* registered.
// europeRemotely, workingNomads, and builtin must not be nil; himalayas may be nil (route returns 500).
// Concrete types may be nil (tests); when set, POST bodies can override per-source settings without mutating bootstrap.
// log is the binary root logger (e.g. from logging.NewRoot); use zerolog.Nop() in tests.
func NewHTTPHandler(
	europeRemotely, workingNomads, himalayasColl, builtinColl collectors.Collector,
	remotifyEuropeColl, weWorkRemotelyColl, wellfoundColl, vueJobsColl, golangCafeColl collectors.Collector,
	workingNomadsConcrete *workingnomads.WorkingNomads,
	europeRemotelyConcrete *europeremotely.EuropeRemotely,
	himalayasConcrete *himalayas.Himalayas,
	builtinConcrete *builtin.BuiltIn,
	remotifyEuropeConcrete *remotifyeurope.RemotifyEurope,
	wellfoundConcrete *wellfound.Wellfound,
	vueJobsConcrete *vuejobs.VueJobs,
	golangCafeConcrete *golangcafe.GolangCafe,
	log zerolog.Logger,
) *HTTPHandler {
	h := &HTTPHandler{
		mux:                    http.NewServeMux(),
		log:                    log,
		europeRemotely:         europeRemotely,
		workingNomads:          workingNomads,
		himalayas:              himalayasColl,
		builtin:                builtinColl,
		remotifyEurope:         remotifyEuropeColl,
		weWorkRemotely:         weWorkRemotelyColl,
		wellfound:              wellfoundColl,
		vueJobs:                vueJobsColl,
		golangCafe:             golangCafeColl,
		workingNomadsConcrete:  workingNomadsConcrete,
		europeRemotelyConcrete: europeRemotelyConcrete,
		himalayasConcrete:      himalayasConcrete,
		builtinConcrete:        builtinConcrete,
		remotifyEuropeConcrete: remotifyEuropeConcrete,
		wellfoundConcrete:      wellfoundConcrete,
		vueJobsConcrete:        vueJobsConcrete,
		golangCafeConcrete:     golangCafeConcrete,
	}
	h.registerRoutes()
	h.chain = logging.RequestIDMiddleware(h.mux)
	return h
}

func (h *HTTPHandler) registerRoutes() {
	h.mux.HandleFunc("GET /health", h.health)
	h.mux.HandleFunc(PathEuropeRemotely, h.postEuropeRemotely)
	h.mux.HandleFunc(PathWorkingNomads, h.postWorkingNomads)
	h.mux.HandleFunc(PathHimalayas, h.postHimalayas)
	h.mux.HandleFunc(PathBuiltin, h.postBuiltin)
	h.mux.HandleFunc(PathRemotifyEurope, h.postRemotifyEurope)
	h.mux.HandleFunc(PathWeWorkRemotely, h.postWeWorkRemotely)
	h.mux.HandleFunc(PathWellfound, h.postWellfound)
	h.mux.HandleFunc(PathVueJobs, h.postVueJobs)
	h.mux.HandleFunc(PathGolangCafe, h.postGolangCafe)
}

// ServeHTTP applies request-id middleware then dispatches to registered routes.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.chain.ServeHTTP(w, r)
}
