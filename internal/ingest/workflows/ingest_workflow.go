package ingest_workflows

import (
	"time"

	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	ingest_activities "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// IngestSourceWorkflowName is the registered workflow type for per-source ingest (006 / 003 patterns).
const IngestSourceWorkflowName = "IngestSourceWorkflow"

// IngestSourceWorkflow loads jobs from one 005 source with Redis coordination (activity implements lock/cooldown/watermark).
// Callers must set IngestSourceInput.SlotID (non-zero UUID) so watermarks stay slot-scoped (006 v2).
func IngestSourceWorkflow(ctx workflow.Context, in ingestschema.IngestSourceInput) (ingestschema.IngestSourceOutput, error) {
	workflow.GetLogger(ctx).Info("ingest source workflow start",
		logging.FieldWorkflow, IngestSourceWorkflowName,
		logging.FieldSlotID, in.SlotID.String(),
		logging.FieldSourceID, in.SourceID,
	)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    20 * time.Minute,
		ScheduleToCloseTimeout: 25 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
		},
	})

	var out ingestschema.IngestSourceOutput
	if err := workflow.ExecuteActivity(ctx, ingest_activities.RunIngestSourceActivityName, in).Get(ctx, &out); err != nil {
		workflow.GetLogger(ctx).Error("RunIngestSource activity failed",
			logging.FieldWorkflow, IngestSourceWorkflowName,
			logging.FieldSlotID, in.SlotID.String(),
			logging.FieldSourceID, in.SourceID,
			"error", err,
		)
		return ingestschema.IngestSourceOutput{}, err
	}
	return out, nil
}
