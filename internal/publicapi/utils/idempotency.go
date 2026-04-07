package utils

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

var (
	// ErrIdempotencyKeyMissing is returned when the Idempotency-Key header is absent or blank.
	ErrIdempotencyKeyMissing = errors.New("idempotency key required")
	// ErrIdempotencyKeyInvalid is returned when Idempotency-Key is not a non-nil UUID.
	ErrIdempotencyKeyInvalid = errors.New("idempotency key must be a non-nil UUID")
)

// ParseIdempotencyKeyHeader reads Idempotency-Key (required for POST /slots) as a UUID.
func ParseIdempotencyKeyHeader(r *http.Request) (uuid.UUID, error) {
	v := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if v == "" {
		return uuid.Nil, ErrIdempotencyKeyMissing
	}
	u, err := uuid.Parse(v)
	if err != nil || u == uuid.Nil {
		return uuid.Nil, ErrIdempotencyKeyInvalid
	}
	return u, nil
}
