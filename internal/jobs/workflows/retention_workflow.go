package jobs_workflows

import (
	"time"

	jobsschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	jobs_activities "github.com/andrewmysliuk/jobhound_core/internal/jobs/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// JobRetentionWorkflowName is the workflow type for hard-deleting stale jobs (006 / retention-jobs.md).
const JobRetentionWorkflowName = "JobRetentionWorkflow"

// JobRetentionWorkflow runs the retention activity (same semantics as cmd/retention run).
func JobRetentionWorkflow(ctx workflow.Context) (*jobsschema.JobRetentionOutput, error) {
	workflow.GetLogger(ctx).Info("job retention workflow start", logging.FieldWorkflow, JobRetentionWorkflowName)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    15 * time.Minute,
		ScheduleToCloseTimeout: 20 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
		},
	})
	var out jobsschema.JobRetentionOutput
	if err := workflow.ExecuteActivity(ctx, jobs_activities.RunJobRetentionActivityName).Get(ctx, &out); err != nil {
		workflow.GetLogger(ctx).Error("RunJobRetention activity failed", logging.FieldWorkflow, JobRetentionWorkflowName, "error", err)
		return nil, err
	}
	return &out, nil
}
