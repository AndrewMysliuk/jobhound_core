// Package pipeline defines the ingest pipeline’s public contracts: collectors, stage rule types
// (BroadFilterRules, KeywordRules), stage-3 scoring via internal/llm.Scorer, dedup, persistence hooks, and notification.
// Pure stage implementations (broad filter, keywords, ScoreJobs batching) live in internal/pipeline/utils.
// Orchestration lives in pipeline/impl; LLM test doubles in internal/llm/mock; pipeline/mock for collectors/dedup/notify;
// job persistence in internal/jobs/storage.
package pipeline

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

// Collector fetches jobs from one source. Each site implements this.
type Collector interface {
	Name() string
	Fetch(ctx context.Context) ([]domain.Job, error)
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
