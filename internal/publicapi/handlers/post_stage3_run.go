package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) postStage3Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	slotID := stringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), slotID)
	logH := logging.EnrichWithContext(ctx, h.deps.Logger.With().Str(logging.FieldHandler, "postStage3Run").Logger())
	var body schema.Stage3RunRequest
	if !ReadJSON(w, r, logH, &body) {
		return
	}
	if body.MaxJobs < 1 || body.MaxJobs > 100 {
		WriteAPIError(w, http.StatusBadRequest, "validation_error", "max_jobs must be between 1 and 100")
		return
	}
	out, err := h.deps.Slots.RunStage3(ctx, slotID, body.MaxJobs)
	if errors.Is(err, slots.ErrNotFound) {
		WriteAPIError(w, http.StatusNotFound, "not_found", "slot not found")
		return
	}
	if errors.Is(err, slots.ErrStageAlreadyRunning) {
		WriteAPIError(w, http.StatusConflict, "stage_already_running", "stage 3 is already running for this slot")
		return
	}
	if errors.Is(err, slots.ErrNoPipelineRun) {
		WriteAPIError(w, http.StatusUnprocessableEntity, "no_pipeline_run", "run stage 2 before stage 3")
		return
	}
	if errors.Is(err, slots.ErrProfileRequired) {
		WriteAPIError(w, http.StatusUnprocessableEntity, "profile_required", "profile text is required for stage 3")
		return
	}
	if err != nil {
		logH.Error().Err(err).Msg("run stage 3")
		WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	WriteJSON(w, http.StatusAccepted, out)
}
