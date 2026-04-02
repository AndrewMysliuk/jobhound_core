// Package pipeline defines stage rule types (BroadFilterRules, KeywordRules), dedup, persistence hooks, notification,
// and orchestration contracts; collectors.Collector lives in internal/collectors.
// Pure stage implementations (broad filter, keywords, ScoreJobs batching, stage-3 cap selection and score→status mapping) live in internal/pipeline/utils.
// Orchestration lives in pipeline/impl; LLM test doubles in internal/llm/mock; pipeline/mock for dedup/notify;
// job persistence in internal/jobs/storage.
package pipeline

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/google/uuid"
)

// Dedup tracks job IDs already delivered (repository port; Postgres impl later).
type Dedup interface {
	WasSent(ctx context.Context, jobID string) (bool, error)
	MarkSent(ctx context.Context, jobID string) error
}

// Notifier delivers scored jobs (e.g. Telegram).
type Notifier interface {
	Send(ctx context.Context, jobs []domain.ScoredJob) error
}

// PipelineRunRepository persists pipeline_runs and pipeline_run_jobs (007).
type PipelineRunRepository interface {
	// CreateRun inserts pipeline_runs. slotID may be nil for legacy rows; otherwise persisted on the run (007 §4).
	CreateRun(ctx context.Context, slotID *uuid.UUID) (pipelineRunID int64, err error)
	// SetBroadFilterKeyHash stores the SHA-256 hex broad filter key on the run (006); empty hash is a no-op.
	SetBroadFilterKeyHash(ctx context.Context, pipelineRunID int64, hash string) error
	SetRunJobStatus(ctx context.Context, pipelineRunID int64, jobID string, status RunJobStatus) error
	// GetRunJobStatus loads the per-run row; ok is false when missing.
	GetRunJobStatus(ctx context.Context, pipelineRunID int64, jobID string) (status RunJobStatus, ok bool, err error)
	// ListPassedStage2JobIDs returns job_id for rows in PASSED_STAGE_2 only (eligible for stage-3 cap per 007 §2).
	ListPassedStage2JobIDs(ctx context.Context, pipelineRunID int64) ([]string, error)
}
