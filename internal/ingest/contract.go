// Package ingest implements cache and ingest coordination (specs/006-cache-and-ingest).
package ingest

import (
	"errors"
	"strings"
)

// TTL defaults per specs/006-cache-and-ingest/contracts/redis-ingest-coordination.md.
// Optional JOBHOUND_INGEST_*_TTL_SEC overrides are applied in internal/config and passed via NewRedisCoordinatorWithTTL.
const (
	IngestLockTTLSeconds     = 600
	IngestCooldownTTLSeconds = 3600
)

var (
	// ErrLockHeld is returned when ingest:lock:{source_id} is already held (SET NX failed).
	ErrLockHeld = errors.New("ingest: lock already held for source")
	// ErrCooldownActive is returned when ingest:cooldown:{source_id} exists and explicit refresh is off.
	ErrCooldownActive = errors.New("ingest: cooldown active for source")
	// ErrEmptySourceID is returned when source_id is empty after normalization.
	ErrEmptySourceID = errors.New("ingest: empty source_id")
	// ErrNilRedisClient is returned when the coordinator was constructed without a client.
	ErrNilRedisClient = errors.New("ingest: nil redis client")
)

// NormalizeSourceID trims whitespace and lowercases ASCII for Redis key segments.
func NormalizeSourceID(sourceID string) string {
	return strings.ToLower(strings.TrimSpace(sourceID))
}
