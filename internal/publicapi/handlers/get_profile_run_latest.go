package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
)

func (h *HTTPHandler) getProfileRunLatest(w http.ResponseWriter, r *http.Request) {
	profileID := apputils.StringsTrimPathValue(r, "profile_id")
	if _, err := h.deps.Profiles.Get(r.Context(), profileID); err != nil {
		if errors.Is(err, profiles.ErrProfileNotFound) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileNotFound, Cause: err})
			return
		}
		if errors.Is(err, profiles.ErrProfileInvalid) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileInvalidDefinition, Cause: err})
			return
		}
		apputils.WriteError(w, err)
		return
	}
	run, err := h.deps.Scoring.LatestRun(r.Context(), profileID)
	if err != nil {
		if errors.Is(err, scoring.ErrNoRun) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeNoRun, Cause: err})
			return
		}
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusOK, schema.ProfileRunResponse{
		RunID:          run.ID,
		ProfileID:      run.ProfileID,
		Status:         run.Status.String(),
		StartedAt:      run.StartedAt,
		FinishedAt:     run.FinishedAt,
		JobsScored:     run.JobsScored,
		SourcesSkipped: run.SourcesSkipped,
	})
}
