package ingest_workflows

import (
	"time"

	ingest_activities "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// IngestSourceWorkflowName is the registered workflow type for per-source ingest (006 / 003 patterns).
const IngestSourceWorkflowName = "IngestSourceWorkflow"

// IngestSourceWorkflow loads jobs from one 005 source with Redis coordination (activity implements lock/cooldown/watermark).
func IngestSourceWorkflow(ctx workflow.Context, in ingest_activities.IngestSourceInput) (ingest_activities.IngestSourceOutput, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    20 * time.Minute,
		ScheduleToCloseTimeout: 25 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
		},
	})

	var out ingest_activities.IngestSourceOutput
	if err := workflow.ExecuteActivity(ctx, ingest_activities.RunIngestSourceActivityName, in).Get(ctx, &out); err != nil {
		return ingest_activities.IngestSourceOutput{}, err
	}
	return out, nil
}
