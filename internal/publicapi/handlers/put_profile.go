package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"gorm.io/gorm"
)

func (h *HTTPHandler) putProfile(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "putProfile")
	if r.Method != http.MethodPut {
		apputils.WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var body schema.ProfilePutRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaProfilePut, &body) {
		return
	}
	out, err := h.deps.Profile.Put(r.Context(), body.Text)
	if err != nil {
		logH.Error().Err(err).Msg("put profile")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", "profile row missing")
			return
		}
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	apputils.WriteJSON(w, http.StatusOK, out)
}
