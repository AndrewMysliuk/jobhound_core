package handlers

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
)

func (h *HTTPHandler) postProfileRun(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "postProfileRun")
	idemKey, err := apputils.ParseIdempotencyKeyHeader(r)
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	if !apputils.RequireEmptyBody(w, r, logH) {
		return
	}
	profileID := apputils.StringsTrimPathValue(r, "profile_id")
	run, err := h.deps.Scoring.StartRun(r.Context(), profileID, idemKey)
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusAccepted, schema.ProfileRunResponse{
		RunID:          run.ID,
		ProfileID:      run.ProfileID,
		Status:         run.Status.String(),
		StartedAt:      run.StartedAt,
		FinishedAt:     run.FinishedAt,
		JobsScored:     run.JobsScored,
		SourcesSkipped: run.SourcesSkipped,
	})
}
