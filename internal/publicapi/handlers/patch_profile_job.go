package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

func (h *HTTPHandler) patchProfileJob(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "patchProfileJob")
	var body schema.PatchUserStatusRequest
	if !apputils.ReadValidatedJSON(w, r, logH, SchemaPatchUserStatus, &body) {
		return
	}
	status, err := scoringschema.FromStringUserStatus(body.UserStatus)
	if err != nil {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidUserStatus, Cause: err})
		return
	}
	profileID := apputils.StringsTrimPathValue(r, "profile_id")
	jobID := apputils.StringsTrimPathValue(r, "job_id")
	if profileID == "" {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileNotFound})
		return
	}
	if jobID == "" {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeJobNotInScope})
		return
	}
	if _, err := h.deps.Profiles.Get(r.Context(), profileID); err != nil {
		if errors.Is(err, profiles.ErrProfileNotFound) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileNotFound, Cause: err})
			return
		}
		if errors.Is(err, profiles.ErrProfileInvalid) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileInvalidDefinition, Cause: err})
			return
		}
		logH.Error().Err(err).Msg("get profile")
		apputils.WriteError(w, err)
		return
	}
	updated, err := h.deps.Scoring.SetUserStatus(r.Context(), profileID, jobID, status)
	if err != nil {
		if errors.Is(err, scoring.ErrJobNotInScope) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeJobNotInScope, Cause: err})
			return
		}
		if errors.Is(err, profiles.ErrProfileNotFound) {
			apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeProfileNotFound, Cause: err})
			return
		}
		logH.Error().Err(err).Msg("set user status")
		apputils.WriteError(w, err)
		return
	}
	page := h.profileJobPage(scoringschema.JobPage{Jobs: []scoringschema.ListedJob{updated}})
	apputils.WriteJSON(w, http.StatusOK, page.Jobs[0])
}
