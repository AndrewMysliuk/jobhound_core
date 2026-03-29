package reference_workflows

import (
	"context"
	"time"

	"go.temporal.io/sdk/client"
)

// StartReferenceDemoWorkflow executes the v0 reference workflow and waits for its result.
// Task queue and timeouts must match the worker and contracts/reference-workflow.md.
func StartReferenceDemoWorkflow(ctx context.Context, c client.Client, taskQueue, workflowID, name string) (string, error) {
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                 workflowID,
		TaskQueue:          taskQueue,
		WorkflowRunTimeout: 2 * time.Minute,
	}, ReferenceWorkflowName, name)
	if err != nil {
		return "", err
	}
	var out string
	if err := run.Get(ctx, &out); err != nil {
		return "", err
	}
	return out, nil
}
