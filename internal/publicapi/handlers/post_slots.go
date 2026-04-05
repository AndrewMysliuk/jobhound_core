package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) postSlots(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "postSlots")
	if r.Method != http.MethodPost {
		apputils.WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var body schema.CreateSlotRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaCreateSlot, &body) {
		return
	}
	card, err := h.deps.Slots.Create(r.Context(), body.Name)
	if errors.Is(err, slots.ErrInvalidSlotName) {
		apputils.WriteAPIError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if errors.Is(err, slots.ErrSlotLimitReached) {
		apputils.WriteSlotLimitReached(w, err.Error())
		return
	}
	if err != nil {
		logH.Error().Err(err).Msg("create slot")
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	apputils.WriteJSON(w, http.StatusCreated, card)
}
