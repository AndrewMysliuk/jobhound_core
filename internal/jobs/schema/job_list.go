package schema

import (
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// Stage1StatusPassed is the jobs.stage1_status value after broad stage 1 (007 pipeline-run-job-status.md §3).
const Stage1StatusPassed = "PASSED_STAGE_1"

// ListBucket filters GET …/stages/2|3/jobs (009). ListBucketAll means no bucket query parameter.
type ListBucket uint8

const (
	ListBucketAll ListBucket = iota
	ListBucketPassed
	ListBucketFailed
)

// JobListEntry is one job row plus slot first_seen_at for paginated slot job lists (009).
type JobListEntry struct {
	Job             jobdata.Job
	FirstSeenAt     time.Time
	Stage3Rationale *string // from pipeline_run_jobs when listing stage 3; nil otherwise
}
