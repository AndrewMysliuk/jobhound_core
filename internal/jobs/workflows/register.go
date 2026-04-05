package jobs_workflows

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	jobs_activities "github.com/andrewmysliuk/jobhound_core/internal/jobs/workflows/activities"
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// RetentionWorkerDeps configures job retention workflow + activity registration.
type RetentionWorkerDeps struct {
	Clock func() time.Time
	Jobs  jobs.JobRepository
	Log   zerolog.Logger
}

// RegisterRetention registers JobRetentionWorkflow and RunJobRetention when Jobs is non-nil.
func RegisterRetention(w worker.Worker, deps RetentionWorkerDeps) {
	if w == nil || deps.Jobs == nil {
		return
	}
	acts := &jobs_activities.RetentionActivities{
		Clock: deps.Clock,
		Jobs:  deps.Jobs,
		Log:   deps.Log,
	}
	w.RegisterActivityWithOptions(acts.RunJobRetention, activity.RegisterOptions{
		Name: jobs_activities.RunJobRetentionActivityName,
	})
	w.RegisterWorkflowWithOptions(JobRetentionWorkflow, workflow.RegisterOptions{
		Name: JobRetentionWorkflowName,
	})
}
