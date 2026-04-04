package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) postSlots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var body schema.CreateSlotRequest
	if !ReadJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		WriteAPIError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	card, err := h.deps.Slots.Create(r.Context(), body.Name)
	if errors.Is(err, slots.ErrInvalidSlotName) {
		WriteAPIError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if errors.Is(err, slots.ErrSlotLimitReached) {
		WriteSlotLimitReached(w, err.Error())
		return
	}
	if err != nil {
		WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, card)
}
