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

// Notifier delivers scored jobs to a downstream channel (orchestration-owned; not used for MVP push).
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
	// ListPassedStage2JobIDs returns job_id for rows in PASSED_STAGE_2 only (eligible for stage-3 cap).
	// Ordering matches 008: jobs.posted_at descending (NULLs last), then job_id ascending for ties.
	ListPassedStage2JobIDs(ctx context.Context, pipelineRunID int64) ([]string, error)

	// InvalidateStage3SnapshotsForSlot resets terminal stage-3 outcomes to PASSED_STAGE_2 for every
	// pipeline_run_jobs row tied to pipeline_runs.slot_id = slotID (008 filter invalidation: stage-3 rules only).
	// REJECTED_STAGE_2 and PASSED_STAGE_2 rows are unchanged. Returns the number of rows updated.
	InvalidateStage3SnapshotsForSlot(ctx context.Context, slotID uuid.UUID) (updated int64, err error)
	// InvalidateStage2And3SnapshotsForSlot deletes all pipeline_runs for the slot (CASCADE removes pipeline_run_jobs).
	// Use when stage-2 (keyword) rules change (008: stage 3 depends on stage 2).
	InvalidateStage2And3SnapshotsForSlot(ctx context.Context, slotID uuid.UUID) (runsDeleted int64, err error)
}
