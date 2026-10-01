package handlers

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
)

func (h *HTTPHandler) postSlots(w http.ResponseWriter, r *http.Request) {
	logH := h.routeLog(r, "postSlots")
	idemKey, err := apputils.ParseIdempotencyKeyHeader(r)
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	var body schema.CreateSlotRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaCreateSlot, &body) {
		return
	}
	res, err := h.deps.Slots.Create(r.Context(), slotschema.CreateSlotParams{Name: body.Name, IdempotencyKey: idemKey})
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	status := http.StatusOK
	if res.Created {
		status = http.StatusCreated
	}
	apputils.WriteJSON(w, status, res.Card)
}
