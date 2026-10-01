package handlers

import (
	"net/http"

	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
)

func (h *HTTPHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	out, err := h.deps.Profile.Get(r.Context())
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusOK, out)
}
