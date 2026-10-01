package handlers

import (
	"net/http"

	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
)

func (h *HTTPHandler) getSlots(w http.ResponseWriter, r *http.Request) {
	resp, err := h.deps.Slots.List(r.Context())
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusOK, resp)
}
