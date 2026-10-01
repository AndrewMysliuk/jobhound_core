package handlers

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
)

func (h *HTTPHandler) putProfile(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "putProfile")
	var body schema.ProfilePutRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaProfilePut, &body) {
		return
	}
	out, err := h.deps.Profile.Put(r.Context(), body.Text)
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusOK, out)
}
