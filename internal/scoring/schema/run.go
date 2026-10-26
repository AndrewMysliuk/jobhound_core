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

	CompaniesDiscovered  int
	CompaniesMerged      int
	CompaniesBlocked     int
	CompaniesResolved    int
	CompaniesUnsupported int
	CompaniesPolled      int
	PollsNotModified     int
	PollsPartial         int
	PollFailures         int
	JobsUpsertedATS      int
	JobsClosed           int
}

// RunATSCounters is a delta applied to the companies/ATS counters on a run row.
type RunATSCounters struct {
	CompaniesDiscovered  int
	CompaniesMerged      int
	CompaniesBlocked     int
	CompaniesResolved    int
	CompaniesUnsupported int
	CompaniesPolled      int
	PollsNotModified     int
	PollsPartial         int
	PollFailures         int
	JobsUpsertedATS      int
	JobsClosed           int
}
