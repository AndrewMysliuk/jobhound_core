package manual_workflows

import (
	"context"
	"time"

	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	"go.temporal.io/sdk/client"
)

// DefaultManualSlotRunWorkflowTimeout bounds the parent workflow wait; must cover parallel ingest children (25m) plus stage 2/3.
const DefaultManualSlotRunWorkflowTimeout = 45 * time.Minute

// StartManualSlotRunWorkflow runs [ManualSlotRunWorkflow] to completion and returns the aggregate.
// Pass taskQueue from [github.com/andrewmysliuk/jobhound_core/internal/config.Temporal.TaskQueue] (default "jobhound");
// namespace is configured on the Temporal client ([client.Dial]).
func StartManualSlotRunWorkflow(ctx context.Context, c client.Client, taskQueue, workflowID string, in manualschema.ManualSlotRunWorkflowInput) (manualschema.ManualSlotRunAggregate, error) {
	if err := in.Validate(); err != nil {
		return manualschema.ManualSlotRunAggregate{}, err
	}
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                 workflowID,
		TaskQueue:          taskQueue,
		WorkflowRunTimeout: DefaultManualSlotRunWorkflowTimeout,
	}, manualschema.ManualSlotRunWorkflowName, in)
	if err != nil {
		return manualschema.ManualSlotRunAggregate{}, err
	}
	var out manualschema.ManualSlotRunAggregate
	if err := run.Get(ctx, &out); err != nil {
		return manualschema.ManualSlotRunAggregate{}, err
	}
	return out, nil
}
