package schema

import (
	"time"

	"github.com/google/uuid"
)

// Run is one profile_runs row.
type Run struct {
	ID             int64
	ProfileID      string
	Status         RunStatus
	StartedAt      time.Time
	FinishedAt     *time.Time
	JobsScored     int
	SourcesSkipped int
	IdempotencyKey uuid.UUID
}
