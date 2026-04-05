package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) postStage3Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apputils.WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	slotID := apputils.StringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), slotID)
	logH := logging.EnrichWithContext(ctx, h.deps.Logger.With().Str(logging.FieldHandler, "postStage3Run").Logger())
	var body schema.Stage3RunRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaStage3Run, &body) {
		return
	}
	out, err := h.deps.Slots.RunStage3(ctx, slotID, body.MaxJobs)
	if errors.Is(err, slots.ErrNotFound) {
		apputils.WriteAPIError(w, http.StatusNotFound, "not_found", "slot not found")
		return
	}
	if errors.Is(err, slots.ErrStageAlreadyRunning) {
		apputils.WriteAPIError(w, http.StatusConflict, "stage_already_running", "stage 3 is already running for this slot")
		return
	}
	if errors.Is(err, slots.ErrNoPipelineRun) {
		apputils.WriteAPIError(w, http.StatusUnprocessableEntity, "no_pipeline_run", "run stage 2 before stage 3")
		return
	}
	if errors.Is(err, slots.ErrProfileRequired) {
		apputils.WriteAPIError(w, http.StatusUnprocessableEntity, "profile_required", "profile text is required for stage 3")
		return
	}
	if err != nil {
		logH.Error().Err(err).Msg("run stage 3")
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	apputils.WriteJSON(w, http.StatusAccepted, out)
}
