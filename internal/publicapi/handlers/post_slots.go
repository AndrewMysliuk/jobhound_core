package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
)

func (h *HTTPHandler) postSlots(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "postSlots")
	if r.Method != http.MethodPost {
		apputils.WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	idemKey, err := apputils.ParseIdempotencyKeyHeader(r)
	if err != nil {
		if errors.Is(err, apputils.ErrIdempotencyKeyMissing) {
			apputils.WriteAPIError(w, http.StatusBadRequest, "idempotency_key_required", "header Idempotency-Key is required")
			return
		}
		apputils.WriteAPIError(w, http.StatusBadRequest, "invalid_idempotency_key", "header Idempotency-Key must be a non-nil UUID")
		return
	}
	var body schema.CreateSlotRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaCreateSlot, &body) {
		return
	}
	res, err := h.deps.Slots.Create(r.Context(), slotschema.CreateSlotParams{Name: body.Name, IdempotencyKey: idemKey})
	if errors.Is(err, slots.ErrInvalidSlotName) {
		apputils.WriteAPIError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if errors.Is(err, slots.ErrInvalidIdempotencyKey) {
		apputils.WriteAPIError(w, http.StatusBadRequest, "invalid_idempotency_key", "header Idempotency-Key must be a non-nil UUID")
		return
	}
	if errors.Is(err, slots.ErrSlotLimitReached) {
		apputils.WriteSlotLimitReached(w, err.Error())
		return
	}
	if errors.Is(err, slots.ErrIdempotencyKeyConflict) {
		apputils.WriteAPIError(w, http.StatusConflict, "idempotency_key_conflict", err.Error())
		return
	}
	if err != nil {
		logH.Error().Err(err).Msg("create slot")
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	status := http.StatusOK
	if res.Created {
		status = http.StatusCreated
	}
	apputils.WriteJSON(w, status, res.Card)
}
