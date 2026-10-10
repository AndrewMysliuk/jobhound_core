// Package jobs is the jobs module: persistence contracts at the root, storage under storage/.
package jobs

import (
	"context"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// JobRepository persists normalized jobs (002 stub; list/search/ingest batch APIs in 006 as needed).
type JobRepository interface {
	Save(ctx context.Context, job jobdata.Job) error
	// SaveIngest inserts a vacancy or merges it into the row with the same id.
	// Sources are unioned. An ATS url and apply_url replace aggregator values.
	// first_seen_at stays; last_seen_at moves. skipped is false because the touch is written.
	SaveIngest(ctx context.Context, job jobdata.Job) (skipped bool, err error)
	GetByID(ctx context.Context, id string) (jobdata.Job, error)
	// ListFirstSeenSince returns jobs whose first_seen_at is at or after since (UTC).
	ListFirstSeenSince(ctx context.Context, since time.Time) ([]jobdata.Job, error)
	// DeleteJobsLastSeenBeforeUTC hard-deletes jobs with last_seen_at strictly before cutoff (UTC).
	DeleteJobsLastSeenBeforeUTC(ctx context.Context, cutoff time.Time) (deleted int64, err error)
}
