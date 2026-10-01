package handlers

import (
	"net/http"

	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
)

func (h *HTTPHandler) postStage2Run(w http.ResponseWriter, r *http.Request) {
	slotID := apputils.StringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), slotID)
	logH := logging.EnrichWithContext(ctx, h.deps.Logger.With().Str(logging.FieldHandler, "postStage2Run").Logger())
	var body schema.Stage2RunRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaStage2Run, &body) {
		return
	}
	if err := pipeutils.ValidateStage2Rules(body.Rules); err != nil {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeValidationFailed, Cause: err})
		return
	}
	out, err := h.deps.Slots.RunStage2(ctx, slotschema.RunStage2Params{SlotID: slotID, Rules: body.Rules})
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusAccepted, out)
}
