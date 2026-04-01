package debughttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

func runCollectorDebug(w http.ResponseWriter, r *http.Request, coll collectors.Collector, wnConcrete *workingnomads.WorkingNomads, erConcrete *europeremotely.EuropeRemotely) {
	if coll == nil {
		http.Error(w, "collector not configured", http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	raw, err := readDebugRequestBody(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := parseCollectorsPOSTBody(raw)
	if err != nil {
		http.Error(w, fmt.Sprintf("JSON body: %v", err), http.StatusBadRequest)
		return
	}
	limit, err := resolveLimit(req.Limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var jobs []domain.Job
	var fetchErr error
	var upstreamFetched int

	isWN := coll.Name() == workingnomads.SourceName && wnConcrete != nil
	isER := coll.Name() == europeremotely.SourceName && erConcrete != nil
	switch {
	case isWN:
		c := *wnConcrete
		applyWorkingNomadsOverrides(&req, &c)
		if limit > 0 {
			c.MaxFetchJobs = limit
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
	default:
		jobs, fetchErr = coll.Fetch(ctx)
		upstreamFetched = len(jobs)
		if limit > 0 && len(jobs) > limit {
			jobs = jobs[:limit]
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if fetchErr != nil {
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
	if !isWN && !isER && upstreamFetched > len(jobs) {
		resp.UpstreamFetched = upstreamFetched
	}

	resp.Jobs = make([]jobDebugJSON, 0, len(jobs))
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, jobToDebugJSON(j))
	}
	_ = json.NewEncoder(w).Encode(resp)
}
