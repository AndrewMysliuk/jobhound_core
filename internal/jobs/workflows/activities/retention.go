// Package jobs_activities hosts Temporal activities for the jobs module (retention, etc.).
package jobs_activities

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	jobsschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	jobutils "github.com/andrewmysliuk/jobhound_core/internal/jobs/utils"
)

// RunJobRetentionActivityName is the registered activity name for scheduled/manual job retention (006).
const RunJobRetentionActivityName = "RunJobRetentionActivity"

// RetentionActivities holds dependencies for job retention (worker wire-up).
type RetentionActivities struct {
	Clock func() time.Time
	Jobs  jobs.JobRepository
}

// RunJobRetention deletes jobs with created_at older than 7 days (UTC), per retention-jobs.md.
// Dependent pipeline_run_jobs rows are removed by ON DELETE CASCADE on job_id (007 pipeline-run-job-status.md §5, §7);
// no explicit delete is required in application code.
func (a *RetentionActivities) RunJobRetention(ctx context.Context) (*jobsschema.JobRetentionOutput, error) {
	if a == nil || a.Jobs == nil {
		return nil, fmt.Errorf("jobs activities: RunJobRetention requires Jobs repository")
	}
	now := time.Now().UTC()
	if a.Clock != nil {
		now = a.Clock().UTC()
	}
	cutoff := jobutils.CutoffUTC(now)
	n, err := a.Jobs.DeleteJobsCreatedBeforeUTC(ctx, cutoff)
	if err != nil {
		return nil, err
	}
	return &jobsschema.JobRetentionOutput{Deleted: n}, nil
}
