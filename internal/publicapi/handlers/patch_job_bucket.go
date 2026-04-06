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

func (h *HTTPHandler) patchStageJobBucket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		apputils.WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	slotID := apputils.StringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), slotID)
	logH := logging.EnrichWithContext(ctx, h.deps.Logger.With().Str(logging.FieldHandler, "patchStageJobBucket").Logger())
	stageStr := apputils.StringsTrimPathValue(r, "stage")
	stage, ok := apputils.ParseStageDigit(stageStr)
	if !ok || stage != 2 && stage != 3 {
		apputils.WriteAPIError(w, http.StatusBadRequest, "invalid_stage", "stage must be 2 or 3")
		return
	}
	jobID := apputils.StringsTrimPathValue(r, "job_id")
	var body schema.PatchJobBucketRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaPatchJobBucket, &body) {
		return
	}
	out, err := h.deps.Slots.PatchJobBucket(ctx, slotschema.PatchJobBucketParams{SlotID: slotID, Stage: stage, JobID: jobID, Bucket: body.Bucket})
	if errors.Is(err, slots.ErrNotFound) {
		apputils.WriteAPIError(w, http.StatusNotFound, "not_found", "slot or job not found for this stage")
		return
	}
	if err != nil {
		logH.Error().Err(err).Msg("patch job bucket")
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	apputils.WriteJSON(w, http.StatusOK, out)
}
