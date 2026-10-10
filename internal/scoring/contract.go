// Package scoring is the rule-scoring module: contracts at the root, data model under schema/.
package scoring

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// API is the scoring use-case surface. Handlers call it. Implementations do not import Temporal.
type API interface {
	StartRun(ctx context.Context, profileID string, idempotencyKey uuid.UUID) (schema.Run, error)
	LatestRun(ctx context.Context, profileID string) (schema.Run, error)
	ListJobs(ctx context.Context, p schema.ListJobsParams) (schema.JobPage, error)
	SetUserStatus(ctx context.Context, profileID, jobID string, s schema.UserStatus) (schema.ListedJob, error)
}

// RunStarter starts the workflow for a run row that already exists.
// The scoring service calls it. Workflows implement it.
type RunStarter interface {
	Start(ctx context.Context, profileID string, runID int64) error
}

var (
	ErrRunAlreadyRunning      = errors.New("scoring: run already in progress")
	ErrIdempotencyKeyConflict = errors.New("scoring: idempotency key reused for another profile")
	ErrJobNotInScope          = errors.New("scoring: job not scored for this profile")
	ErrNoRun                  = errors.New("scoring: no run for this profile")
)
