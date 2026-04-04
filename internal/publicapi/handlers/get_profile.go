package handlers

import (
	"errors"
	"net/http"

	"gorm.io/gorm"
)

func (h *HTTPHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	out, err := h.deps.Profile.Get(r.Context())
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
