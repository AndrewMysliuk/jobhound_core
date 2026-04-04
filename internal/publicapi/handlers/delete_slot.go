package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) deleteSlot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	id := stringsTrimPathValue(r, "slot_id")
	err := h.deps.Slots.Delete(r.Context(), id)
	if errors.Is(err, slots.ErrNotFound) {
		WriteAPIError(w, http.StatusNotFound, "not_found", "slot not found")
		return
	}
	if err != nil {
		WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
