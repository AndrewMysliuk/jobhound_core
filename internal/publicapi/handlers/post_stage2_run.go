package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	"github.com/rs/zerolog"
)

func (h *HTTPHandler) postStage2Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	slotID := stringsTrimPathValue(r, "slot_id")
	ctx := logging.WithSlotID(r.Context(), slotID)
	logH := logging.EnrichWithContext(ctx, h.deps.Logger.With().Str(logging.FieldHandler, "postStage2Run").Logger())
	raw, ok := readJSONObject(w, r, logH)
	if !ok {
		return
	}
	if _, has := raw["include"]; !has {
		WriteAPIError(w, http.StatusBadRequest, "validation_error", "include is required")
		return
	}
	if _, has := raw["exclude"]; !has {
		WriteAPIError(w, http.StatusBadRequest, "validation_error", "exclude is required")
		return
	}
	var include, exclude []string
	if err := json.Unmarshal(raw["include"], &include); err != nil {
		WriteAPIError(w, http.StatusBadRequest, "validation_error", "include must be a JSON array of strings")
		return
	}
	if err := json.Unmarshal(raw["exclude"], &exclude); err != nil {
		WriteAPIError(w, http.StatusBadRequest, "validation_error", "exclude must be a JSON array of strings")
		return
	}
	out, err := h.deps.Slots.RunStage2(ctx, slotID, include, exclude)
	if errors.Is(err, slots.ErrNotFound) {
		WriteAPIError(w, http.StatusNotFound, "not_found", "slot not found")
		return
	}
	if errors.Is(err, slots.ErrStageAlreadyRunning) {
		WriteAPIError(w, http.StatusConflict, "stage_already_running", "stage 2 is already running for this slot")
		return
	}
	if err != nil {
		logH.Error().Err(err).Msg("run stage 2")
		WriteAPIError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	WriteJSON(w, http.StatusAccepted, out)
}

func readJSONObject(w http.ResponseWriter, r *http.Request, log zerolog.Logger) (map[string]json.RawMessage, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("read stage2 run body")
		WriteAPIError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return nil, false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Error().Err(err).Msg("parse stage2 run json")
		WriteAPIError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return nil, false
	}
	return raw, true
}
