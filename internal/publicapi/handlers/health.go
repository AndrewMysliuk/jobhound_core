package handlers

import (
	"net/http"

	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
)

func (h *HTTPHandler) getHealth(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "getHealth")
	logH.Debug().Msg("health")
	apputils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
