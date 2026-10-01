package handlers

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
)

func (h *HTTPHandler) patchStageJobBucket(w http.ResponseWriter, r *http.Request) {
	slotID := apputils.StringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), slotID)
	logH := logging.EnrichWithContext(ctx, h.deps.Logger.With().Str(logging.FieldHandler, "patchStageJobBucket").Logger())
	stageStr := apputils.StringsTrimPathValue(r, "stage")
	stage, ok := apputils.ParseStageDigit(stageStr)
	if !ok || stage != 2 && stage != 3 {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidStage})
		return
	}
	jobID := apputils.StringsTrimPathValue(r, "job_id")
	var body schema.PatchJobBucketRequest
	if !apputils.ReadValidatedJSON(w, r, logH, schemaPatchJobBucket, &body) {
		return
	}
	out, err := h.deps.Slots.PatchJobBucket(ctx, slotschema.PatchJobBucketParams{SlotID: slotID, Stage: stage, JobID: jobID, Bucket: body.Bucket})
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusOK, out)
}
