// Package jobs is the jobs module: persistence contracts at the root, storage under storage/.
package jobs

import (
	"context"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

// Stage1StatusPassed is the jobs.stage1_status value after broad stage 1 (007 pipeline-run-job-status.md §3).
const Stage1StatusPassed = "PASSED_STAGE_1"

// JobRepository persists normalized jobs (002 stub; list/search/ingest batch APIs in 006 as needed).
type JobRepository interface {
	Save(ctx context.Context, job domain.Job) error
	// SaveIngest upserts after broad stage 1 (006): sets stage1_status to PASSED_STAGE_1, skips DB write
	// when the row already matches on all fields except description and description is unchanged;
	// updates description (and updated_at) only when everything else matches but description differs.
	SaveIngest(ctx context.Context, job domain.Job) (skipped bool, err error)
	GetByID(ctx context.Context, id string) (domain.Job, error)
	// DeleteJobsCreatedBeforeUTC hard-deletes jobs with created_at strictly before cutoff (UTC).
	// Dependent pipeline_run_jobs rows must be removed via ON DELETE CASCADE (007) or equivalent.
	DeleteJobsCreatedBeforeUTC(ctx context.Context, cutoff time.Time) (deleted int64, err error)
}
