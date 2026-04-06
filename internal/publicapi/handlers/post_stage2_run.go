package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
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
	out, err := h.deps.Slots.RunStage2(ctx, slotschema.RunStage2Params{SlotID: slotID, Include: body.Include, Exclude: body.Exclude})
	if errors.Is(err, slots.ErrNotFound) {
		apputils.WriteAPIError(w, http.StatusNotFound, "not_found", "slot not found")
		return
	}
	if errors.Is(err, slots.ErrStageAlreadyRunning) {
		apputils.WriteAPIError(w, http.StatusConflict, "stage_already_running", "stage 2 is already running for this slot")
		return
	}
	if err != nil {
		logH.Error().Err(err).Msg("run stage 2")
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	apputils.WriteJSON(w, http.StatusAccepted, out)
}
