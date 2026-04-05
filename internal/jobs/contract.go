// Package jobs is the jobs module: persistence contracts at the root, storage under storage/.
package jobs

import (
	"context"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	"github.com/google/uuid"
)

// JobRepository persists normalized jobs (002 stub; list/search/ingest batch APIs in 006 as needed).
type JobRepository interface {
	Save(ctx context.Context, job jobdata.Job) error
	// SaveIngest upserts after broad stage 1 (006): sets stage1_status to PASSED_STAGE_1, skips DB write
	// when the row already matches on all fields except description and description is unchanged;
	// updates description (and updated_at) only when everything else matches but description differs.
	SaveIngest(ctx context.Context, job jobdata.Job) (skipped bool, err error)
	GetByID(ctx context.Context, id string) (jobdata.Job, error)
	// DeleteJobsCreatedBeforeUTC hard-deletes jobs with created_at strictly before cutoff (UTC).
	// Dependent pipeline_run_jobs rows must be removed via ON DELETE CASCADE (007) or equivalent.
	DeleteJobsCreatedBeforeUTC(ctx context.Context, cutoff time.Time) (deleted int64, err error)

	// UpsertSlotJob inserts (slot_id, job_id) if absent; no-op when the pair exists (008 slot_jobs).
	// Caller must ensure the job row exists (e.g. after SaveIngest).
	UpsertSlotJob(ctx context.Context, slotID uuid.UUID, jobID string) error
	// ListSlotJobsPassedStage1 returns jobs linked to the slot with stage1_status PASSED_STAGE_1 (008 stage-2 pool).
	ListSlotJobsPassedStage1(ctx context.Context, slotID uuid.UUID) ([]jobdata.Job, error)
	// ListPassedStage2JobsForRun returns full job rows for pipeline_run_jobs in PASSED_STAGE_2 for this run,
	// ordered by jobs.posted_at descending (008 stage-3 batch selection; NULL posted_at last).
	ListPassedStage2JobsForRun(ctx context.Context, pipelineRunID int64) ([]jobdata.Job, error)

	// ListSlotStage1Jobs returns stage-1 pool jobs for the slot (PASSED_STAGE_1 + slot_jobs), sorted posted_at DESC, job_id ASC, paginated.
	ListSlotStage1Jobs(ctx context.Context, slotID uuid.UUID, offset, limit int) ([]schema.JobListEntry, int64, error)
	// ListPipelineRunStage2Jobs returns stage-2 outcomes for the run scoped to the slot (join slot_jobs). bucket filters passed/failed when not ListBucketAll.
	ListPipelineRunStage2Jobs(ctx context.Context, slotID uuid.UUID, pipelineRunID int64, bucket schema.ListBucket, offset, limit int) ([]schema.JobListEntry, int64, error)
	// ListPipelineRunStage3Jobs returns terminal stage-3 rows for the run scoped to the slot. bucket filters when not ListBucketAll.
	ListPipelineRunStage3Jobs(ctx context.Context, slotID uuid.UUID, pipelineRunID int64, bucket schema.ListBucket, offset, limit int) ([]schema.JobListEntry, int64, error)
}
