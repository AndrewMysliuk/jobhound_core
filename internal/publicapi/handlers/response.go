package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
)

const maxJSONBodyBytes = 1 << 20

// WriteJSON sets Content-Type application/json, encodes v, and writes status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteAPIError writes the standard error envelope (400, 404, 409, 422, 500).
// For 500, message is replaced with a generic phrase so callers never leak internals.
func WriteAPIError(w http.ResponseWriter, status int, code, message string) {
	if status == http.StatusInternalServerError {
		message = "internal server error"
	}
	WriteJSON(w, status, schema.APIErrorBody{
		Error: schema.APIErrorDetail{Code: code, Message: strings.TrimSpace(message)},
	})
}

// WriteSlotLimitReached writes POST /slots 409 with top-level limit (contracts/http-public-api.md §4.3).
func WriteSlotLimitReached(w http.ResponseWriter, message string) {
	if strings.TrimSpace(message) == "" {
		message = "slot limit reached"
	}
	WriteJSON(w, http.StatusConflict, schema.SlotLimitReachedBody{
		Error: schema.APIErrorDetail{Code: "slot_limit_reached", Message: message},
		Limit: 3,
	})
}

// ReadJSON decodes a JSON body (max 1 MiB). On failure it writes 400 invalid_json and returns false.
func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		WriteAPIError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return false
	}
	if err := discardExtraJSON(dec); err != nil {
		WriteAPIError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
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
