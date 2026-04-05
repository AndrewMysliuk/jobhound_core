package handlers

import "net/http"

func (h *HTTPHandler) getHealth(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "getHealth")
	logH.Debug().Msg("health")
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
