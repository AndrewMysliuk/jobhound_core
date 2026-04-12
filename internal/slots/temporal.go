package slots

import (
	"context"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

// WorkflowTemporal is the subset of [client.Client] used by [impl.Service] (tests may supply fakes).
type WorkflowTemporal interface {
	ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error)
	DescribeWorkflow(ctx context.Context, workflowID, runID string) (*client.WorkflowExecutionDescription, error)
	GetWorkflowHistory(ctx context.Context, workflowID, runID string, isLongPoll bool, filterType enumspb.HistoryEventFilterType) client.HistoryEventIterator
	TerminateWorkflow(ctx context.Context, workflowID, runID string, reason string, details ...interface{}) error
}
