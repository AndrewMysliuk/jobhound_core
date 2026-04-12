package debughttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/djinni"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/dou"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/rs/zerolog"
)

func runCollectorDebug(w http.ResponseWriter, r *http.Request, logH zerolog.Logger, coll collectors.Collector, wnConcrete *workingnomads.WorkingNomads, erConcrete *europeremotely.EuropeRemotely, douConcrete *dou.DOU, himConcrete *himalayas.Himalayas, djinConcrete *djinni.Djinni, builtinConcrete *builtin.BuiltIn) {
	if coll == nil {
		logH.Error().Msg("collector not configured")
		http.Error(w, "collector not configured", http.StatusInternalServerError)
		return
	}
	logH.Debug().Str(logging.FieldSourceID, coll.Name()).Msg("debug collector fetch")
	ctx := r.Context()
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	raw, err := readDebugRequestBody(w, r)
	if err != nil {
		logH.Error().Err(err).Msg("read request body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := parseCollectorsPOSTBody(raw)
	if err != nil {
		logH.Error().Err(err).Msg("parse json body")
		http.Error(w, fmt.Sprintf("JSON body: %v", err), http.StatusBadRequest)
		return
	}
	limit, err := resolveLimit(req.Limit)
	if err != nil {
		logH.Error().Err(err).Msg("resolve limit")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var jobs []schema.Job
	var fetchErr error
	var upstreamFetched int

	isWN := coll.Name() == workingnomads.SourceName && wnConcrete != nil
	isER := coll.Name() == europeremotely.SourceName && erConcrete != nil
	isDOU := coll.Name() == dou.SourceName && douConcrete != nil
	isHim := coll.Name() == himalayas.SourceName && himConcrete != nil
	isDjin := coll.Name() == djinni.SourceName && djinConcrete != nil
	isBuiltin := coll.Name() == builtin.SourceName && builtinConcrete != nil
	switch {
	case isWN:
		c := *wnConcrete
		applyWorkingNomadsOverrides(&req, &c)
		if limit > 0 {
			c.MaxFetchJobs = limit
		} else if limit == 0 {
			// resolveLimit(0) = unlimited jobs; lift DefaultMaxPages so debug can scrape beyond MVP cap.
			c.MaxPages = -1
		}
		jobs, fetchErr = c.Fetch(ctx)
		upstreamFetched = len(jobs)
	case isDOU:
		c := *douConcrete
		applyDouOverrides(&req, &c)
		if limit > 0 {
			c.MaxJobs = limit
		}
		jobs, fetchErr = c.Fetch(ctx)
		upstreamFetched = len(jobs)
	case isER:
		c := *erConcrete
		applyEuropeRemotelyOverrides(&req, &c)
		if limit > 0 {
			c.MaxJobs = limit
		}
		jobs, fetchErr = c.Fetch(ctx)
		upstreamFetched = len(jobs)
	case isHim:
		c := *himConcrete
		applyHimalayasOverrides(&req, &c)
		if limit > 0 {
			c.MaxFetchJobs = limit
		}
		jobs, fetchErr = c.Fetch(ctx)
		upstreamFetched = len(jobs)
	case isDjin:
		c := *djinConcrete
		applyDjinniOverrides(&req, &c)
		if limit > 0 {
			c.MaxJobs = limit
		}
		jobs, fetchErr = c.Fetch(ctx)
		upstreamFetched = len(jobs)
	case isBuiltin:
		c := *builtinConcrete
		applyBuiltinOverrides(&req, &c)
		if limit > 0 {
			c.MaxJobs = limit
		}
		slotQ := ""
		if req.BuiltinSearch != nil {
			slotQ = strings.TrimSpace(*req.BuiltinSearch)
		}
		if slotQ == "" {
			jobs, fetchErr = c.Fetch(ctx)
		} else {
			jobs, fetchErr = c.FetchWithSlotSearch(ctx, slotQ)
		}
		upstreamFetched = len(jobs)
	default:
		jobs, fetchErr = coll.Fetch(ctx)
		upstreamFetched = len(jobs)
		if limit > 0 && len(jobs) > limit {
			jobs = jobs[:limit]
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if fetchErr != nil {
		logH.Error().Err(fetchErr).Str(logging.FieldSourceID, coll.Name()).Msg("collector fetch failed")
		_ = json.NewEncoder(w).Encode(runCollectorResponse{
			OK:        false,
			Collector: coll.Name(),
			Error:     fetchErr.Error(),
		})
		return
	}

	resp := runCollectorResponse{
		OK:        true,
		Collector: coll.Name(),
		Count:     len(jobs),
	}
	if !isWN && !isER && !isDOU && !isHim && !isDjin && !isBuiltin && upstreamFetched > len(jobs) {
		resp.UpstreamFetched = upstreamFetched
	}

	resp.Jobs = make([]jobDebugJSON, 0, len(jobs))
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, jobToDebugJSON(j))
	}
	_ = json.NewEncoder(w).Encode(resp)
}
