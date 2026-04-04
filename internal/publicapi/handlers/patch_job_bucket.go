package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) patchStageJobBucket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	slotID := stringsTrimPathValue(r, "slot_id")
	stageStr := stringsTrimPathValue(r, "stage")
	stage, ok := parseStageDigit(stageStr)
	if !ok || stage != 2 && stage != 3 {
		WriteAPIError(w, http.StatusBadRequest, "invalid_stage", "stage must be 2 or 3")
		return
	}
	jobID := stringsTrimPathValue(r, "job_id")
	var body schema.PatchJobBucketRequest
	if !ReadJSON(w, r, &body) {
		return
	}
	if !body.Bucket.Valid() {
		WriteAPIError(w, http.StatusBadRequest, "invalid_body", "bucket must be passed or failed")
		return
	}
	out, err := h.deps.Slots.PatchJobBucket(r.Context(), slotID, stage, jobID, body.Bucket)
	if errors.Is(err, slots.ErrNotFound) {
		WriteAPIError(w, http.StatusNotFound, "not_found", "slot or job not found for this stage")
		return
	}
	if err != nil {
		WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, out)
}
