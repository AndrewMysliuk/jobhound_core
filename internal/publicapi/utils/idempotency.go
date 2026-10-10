package utils

import (
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/google/uuid"
)

// ParseIdempotencyKeyHeader reads Idempotency-Key as a non-nil UUID.
// A missing or nil key is an APIError with the matching HTTP code.
func ParseIdempotencyKeyHeader(r *http.Request) (uuid.UUID, error) {
	v := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if v == "" {
		return uuid.Nil, schema.APIError{Code: schema.APIErrorCodeIdempotencyKeyRequired}
	}
	u, err := uuid.Parse(v)
	if err != nil || u == uuid.Nil {
		return uuid.Nil, schema.APIError{Code: schema.APIErrorCodeInvalidIdempotencyKey, Cause: err}
	}
	return u, nil
}
