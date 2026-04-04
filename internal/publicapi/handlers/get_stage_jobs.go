package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
)

func (h *HTTPHandler) getStageJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	slotID := stringsTrimPathValue(r, "slot_id")
	stageStr := stringsTrimPathValue(r, "stage")
	stage, ok := parseStageDigit(stageStr)
	if !ok || stage < 1 || stage > 3 {
		WriteAPIError(w, http.StatusBadRequest, "invalid_stage", "stage must be 1, 2, or 3")
		return
	}
	page, limit, bucket, ok := parseJobListQuery(r.URL.Query())
	if !ok {
		WriteAPIError(w, http.StatusBadRequest, "invalid_query", "invalid page, limit, or bucket query")
		return
	}
	if stage == 1 && bucket != "" {
		WriteAPIError(w, http.StatusBadRequest, "invalid_query", "bucket is only allowed for stages 2 and 3")
		return
	}
	resp, err := h.deps.Slots.ListJobs(r.Context(), slotID, stage, page, limit, bucket)
	if errors.Is(err, slots.ErrNotFound) {
		WriteAPIError(w, http.StatusNotFound, "not_found", "slot not found")
		return
	}
	if errors.Is(err, slots.ErrInvalidJobListQuery) {
		WriteAPIError(w, http.StatusBadRequest, "invalid_query", "invalid bucket query parameter")
		return
	}
	if err != nil {
		WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, resp)
}

func parseStageDigit(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 1 || s[0] < '1' || s[0] > '3' {
		return 0, false
	}
	return int(s[0] - '0'), true
}

func parseJobListQuery(q map[string][]string) (page, limit int, bucket string, ok bool) {
	page = 1
	if vs := q["page"]; len(vs) > 0 && strings.TrimSpace(vs[0]) != "" {
		p, err := strconv.Atoi(strings.TrimSpace(vs[0]))
		if err != nil || p < 1 {
			return 0, 0, "", false
		}
		page = p
	}
	limit = schema.DefaultJobListLimit
	if vs := q["limit"]; len(vs) > 0 && strings.TrimSpace(vs[0]) != "" {
		l, err := strconv.Atoi(strings.TrimSpace(vs[0]))
		if err != nil || l < 1 || l > schema.MaxJobListLimit {
			return 0, 0, "", false
		}
		limit = l
	}
	if vs := q["bucket"]; len(vs) > 0 {
		bucket = strings.TrimSpace(vs[0])
	}
	return page, limit, bucket, true
}
