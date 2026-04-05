package handlers

import (
	"net/http"

	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
)

func (h *HTTPHandler) getSlots(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "getSlots")
	if r.Method != http.MethodGet {
		apputils.WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	resp, err := h.deps.Slots.List(r.Context())
	if err != nil {
		logH.Error().Err(err).Msg("list slots")
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	apputils.WriteJSON(w, http.StatusOK, resp)
}
