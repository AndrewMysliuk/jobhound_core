package handlers

import (
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
)

func (h *HTTPHandler) getStageJobs(w http.ResponseWriter, r *http.Request) {
	slotID := apputils.StringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), slotID)
	stageStr := apputils.StringsTrimPathValue(r, "stage")
	stage, ok := apputils.ParseStageDigit(stageStr)
	if !ok || stage < 1 || stage > 3 {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidStage})
		return
	}
	page, limit, statusQ, debug, ok := apputils.ParseJobListQuery(r.URL.Query())
	if !ok || (stage == 1 && statusQ != "") {
		apputils.WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidQuery})
		return
	}
	resp, err := h.deps.Slots.ListJobs(ctx, slotschema.ListJobsParams{
		SlotID: slotID, Stage: stage, Page: page, Limit: limit, StatusQuery: statusQ, Stage2Debug: stage == 2 && debug,
	})
	if err != nil {
		apputils.WriteError(w, err)
		return
	}
	apputils.WriteJSON(w, http.StatusOK, resp)
}
