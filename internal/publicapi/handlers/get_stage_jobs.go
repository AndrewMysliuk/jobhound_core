package handlers

import (
	"errors"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	apputils "github.com/andrewmysliuk/jobhound_core/internal/publicapi/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) getStageJobs(w http.ResponseWriter, r *http.Request) {
	slotID := apputils.StringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), slotID)
	logH := logging.EnrichWithContext(ctx, h.deps.Logger.With().Str(logging.FieldHandler, "getStageJobs").Logger())
	stageStr := apputils.StringsTrimPathValue(r, "stage")
	stage, ok := apputils.ParseStageDigit(stageStr)
	if !ok || stage < 1 || stage > 3 {
		apputils.WriteAPIError(w, http.StatusBadRequest, "invalid_stage", "stage must be 1, 2, or 3")
		return
	}
	page, limit, statusQ, ok := apputils.ParseJobListQuery(r.URL.Query())
	if !ok {
		apputils.WriteAPIError(w, http.StatusBadRequest, "invalid_query", "invalid page, limit, or status query")
		return
	}
	if stage == 1 && statusQ != "" {
		apputils.WriteAPIError(w, http.StatusBadRequest, "invalid_query", "status filter is only allowed for stages 2 and 3")
		return
	}
	resp, err := h.deps.Slots.ListJobs(ctx, slotID, stage, page, limit, statusQ)
	if errors.Is(err, slots.ErrNotFound) {
		apputils.WriteAPIError(w, http.StatusNotFound, "not_found", "slot not found")
		return
	}
	if errors.Is(err, slots.ErrInvalidJobListQuery) {
		apputils.WriteAPIError(w, http.StatusBadRequest, "invalid_query", "invalid status query parameter (use PASSED_STAGE_2, REJECTED_STAGE_2, PASSED_STAGE_3, or REJECTED_STAGE_3)")
		return
	}
	if err != nil {
		logH.Error().Err(err).Msg("list stage jobs")
		apputils.WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	apputils.WriteJSON(w, http.StatusOK, resp)
}
