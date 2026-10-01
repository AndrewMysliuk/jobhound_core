package handlers

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
)

func (h *HTTPHandler) postStage3Run(w http.ResponseWriter, r *http.Request) {
	slotID := apputils.StringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), slotID)
	logH := logging.EnrichWithContext(ctx, h.deps.Logger.With().Str(logging.FieldHandler, "postStage3Run").Logger())
	var body schema.Stage3RunRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaStage3Run, &body) {
		return
	}
	out, err := h.deps.Slots.RunStage3(ctx, slotschema.RunStage3Params{SlotID: slotID, MaxJobs: body.MaxJobs})
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusAccepted, out)
}
