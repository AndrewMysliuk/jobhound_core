package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) postSlots(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "postSlots")
	if r.Method != http.MethodPost {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var body schema.CreateSlotRequest
	if !ReadJSON(w, r, logH, &body) {
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
		logH.Error().Err(err).Msg("create slot")
		WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, card)
}
