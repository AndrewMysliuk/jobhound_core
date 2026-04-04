package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) getSlot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	id := stringsTrimPathValue(r, "slot_id")
	card, err := h.deps.Slots.Get(r.Context(), id)
	if errors.Is(err, slots.ErrNotFound) {
		WriteAPIError(w, http.StatusNotFound, "not_found", "slot not found")
		return
	}
	if err != nil {
		WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, card)
}
