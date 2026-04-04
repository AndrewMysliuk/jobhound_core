package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"gorm.io/gorm"
)

func (h *HTTPHandler) putProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var body schema.ProfilePutRequest
	if !ReadJSON(w, r, &body) {
		return
	}
	out, err := h.deps.Profile.Put(r.Context(), body.Text)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			WriteAPIError(w, http.StatusInternalServerError, "internal_error", "profile row missing")
			return
		}
		WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, out)
}
