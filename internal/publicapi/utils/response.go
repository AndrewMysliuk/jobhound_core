package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

const maxJSONBodyBytes = 1 << 20

// WriteJSON sets Content-Type application/json, encodes v, and writes status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type sentinelCode struct {
	err  error
	code schema.APIErrorCode
}

// sentinelCodes is the contracts order. Slot cap is the same code with a different body.
var sentinelCodes = []sentinelCode{
	{slots.ErrNotFound, schema.APIErrorCodeSlotNotFound},
	{slots.ErrSlotLimitReached, schema.APIErrorCodeSlotLimitReached},
	{slots.ErrInvalidSlotName, schema.APIErrorCodeValidationFailed},
	{slots.ErrIdempotencyKeyConflict, schema.APIErrorCodeIdempotencyKeyConflict},
	{slots.ErrInvalidIdempotencyKey, schema.APIErrorCodeInvalidIdempotencyKey},
	{slots.ErrStageAlreadyRunning, schema.APIErrorCodeStageAlreadyRunning},
	{slots.ErrNoPipelineRun, schema.APIErrorCodeNoPipelineRun},
	{slots.ErrProfileRequired, schema.APIErrorCodeProfileRequired},
	{slots.ErrInvalidJobListQuery, schema.APIErrorCodeInvalidQuery},
	{pipeline.ErrManualPatchNotInScope, schema.APIErrorCodeJobNotInScope},
}

// WriteError resolves err to a registered code and writes the envelope.
// A typed APIError wins, then a sentinel, then INTERNAL.UNEXPECTED.
// The 500 message is only the registry sentence.
func WriteError(w http.ResponseWriter, err error) {
	var apiErr schema.APIError
	if errors.As(err, &apiErr) {
		spec, ok := schema.Lookup(apiErr.Code)
		if !ok {
			writeUnexpected(w, err)
			return
		}
		if apiErr.Cause != nil {
			logCause(spec.Code, apiErr.Cause)
		}
		writeSpec(w, spec)
		return
	}
	for _, row := range sentinelCodes {
		if errors.Is(err, row.err) {
			if row.code == schema.APIErrorCodeSlotLimitReached {
				WriteSlotLimitReached(w)
				return
			}
			spec, ok := schema.Lookup(row.code)
			if !ok {
				writeUnexpected(w, err)
				return
			}
			writeSpec(w, spec)
			return
		}
	}
	writeUnexpected(w, err)
}

func writeUnexpected(w http.ResponseWriter, err error) {
	logCause(schema.APIErrorCodeUnexpected, err)
	spec, ok := schema.Lookup(schema.APIErrorCodeUnexpected)
	if !ok {
		WriteJSON(w, http.StatusInternalServerError, schema.APIErrorBody{
			Error: schema.APIErrorDetail{Code: schema.APIErrorCodeUnexpected.String(), Message: "Internal server error."},
		})
		return
	}
	writeSpec(w, spec)
}

func writeSpec(w http.ResponseWriter, spec schema.APIErrorSpec) {
	WriteJSON(w, spec.Status, schema.APIErrorBody{
		Error: schema.APIErrorDetail{Code: spec.Code.String(), Message: spec.Message},
	})
}

func logCause(code schema.APIErrorCode, cause error) {
	ev := zlog.Error()
	if code == schema.APIErrorCodeValidationFailed {
		ev = zlog.Warn()
	}
	ev = ev.Str("code", code.String())
	if cause != nil {
		ev = ev.Err(cause)
	}
	ev.Msg("api error")
}

// WriteSlotLimitReached writes POST /slots 409 with top-level limit.
func WriteSlotLimitReached(w http.ResponseWriter) {
	spec, ok := schema.Lookup(schema.APIErrorCodeSlotLimitReached)
	if !ok {
		writeUnexpected(w, schema.APIError{Code: schema.APIErrorCodeSlotLimitReached})
		return
	}
	WriteJSON(w, spec.Status, schema.SlotLimitReachedBody{
		Error: schema.APIErrorDetail{Code: spec.Code.String(), Message: spec.Message},
		Limit: 3,
	})
}

// ReadJSON decodes a JSON body (max 1 MiB). On failure it writes HTTP.INVALID_JSON and returns false.
func ReadJSON(w http.ResponseWriter, r *http.Request, log zerolog.Logger, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		log.Error().Err(err).Msg("decode json body")
		WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidJSON})
		return false
	}
	if err := discardExtraJSON(dec); err != nil {
		log.Error().Err(err).Msg("discard extra json")
		WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidJSON})
		return false
	}
	return true
}

// ReadValidatedJSON reads the body (max 1 MiB), validates instance against JSON Schema,
// then decodes into dst with DisallowUnknownFields. On failure writes 400 and returns false.
func ReadValidatedJSON(w http.ResponseWriter, r *http.Request, log zerolog.Logger, schemaBytes []byte, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("read json body")
		WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidJSON})
		return false
	}
	var instance any
	if err := json.Unmarshal(body, &instance); err != nil {
		log.Error().Err(err).Msg("decode json for validation")
		WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidJSON})
		return false
	}
	if err := ValidateJSONInstance(schemaBytes, instance); err != nil {
		log.Warn().Err(err).Msg("json schema validation")
		WriteError(w, schema.APIError{Code: schema.APIErrorCodeValidationFailed})
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		log.Error().Err(err).Msg("decode json body")
		WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidJSON})
		return false
	}
	if err := discardExtraJSON(dec); err != nil {
		log.Error().Err(err).Msg("discard extra json")
		WriteError(w, schema.APIError{Code: schema.APIErrorCodeInvalidJSON})
		return false
	}
	return true
}

func discardExtraJSON(dec *json.Decoder) error {
	if err := dec.Decode(&struct{}{}); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return errors.New("trailing JSON values")
}
