package handlers

import (
	"errors"
	"net/http"

	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"gorm.io/gorm"
)

func (h *HTTPHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "getProfile")
	if r.Method != http.MethodGet {
		apputils.WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	out, err := h.deps.Profile.Get(r.Context())
	if err != nil {
		logH.Error().Err(err).Msg("get profile")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", "profile row missing")
			return
		}
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	apputils.WriteJSON(w, http.StatusOK, out)
}
