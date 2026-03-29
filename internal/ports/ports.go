// Package ports defines I/O boundaries for the ingest pipeline. Domain services
// depend only on repository interfaces (e.g. Dedup); implementations live under
// internal/adapters and internal/.../storage.
package ports

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

// Collector fetches jobs from one source. Each site implements this.
type Collector interface {
	Name() string
	Fetch(ctx context.Context) ([]domain.Job, error)
}

// Filter performs keyword include/exclude without LLM (stage 2).
type Filter interface {
	Apply(jobs []domain.Job) []domain.Job
}

// Scorer runs LLM scoring on the post-filter pool (stage 3).
type Scorer interface {
	Score(ctx context.Context, jobs []domain.Job) ([]domain.ScoredJob, error)
}

// Dedup tracks job IDs already delivered (repository port; Postgres impl later).
type Dedup interface {
	WasSent(ctx context.Context, jobID string) (bool, error)
	MarkSent(ctx context.Context, jobID string) error
}

// Notifier delivers scored jobs (e.g. Telegram).
type Notifier interface {
	Send(ctx context.Context, jobs []domain.ScoredJob) error
}

// SessionProvider supplies browser/session state for headless collectors.
type SessionProvider interface {
	CookieFilePath() string
}
