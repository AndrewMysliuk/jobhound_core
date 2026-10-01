package slots

import (
	"context"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

// Aliases so impl does not import the Temporal SDK. Types are identical to the SDK types.
type (
	StartWorkflowOptions         = client.StartWorkflowOptions
	WorkflowRun                  = client.WorkflowRun
	WorkflowExecutionDescription = client.WorkflowExecutionDescription
	WorkflowExecutionMetadata    = client.WorkflowExecutionMetadata
	HistoryEventIterator         = client.HistoryEventIterator
)

// WorkflowTemporal is the subset of [client.Client] used by [impl.Service] (tests may supply fakes).
type WorkflowTemporal interface {
	ExecuteWorkflow(ctx context.Context, options StartWorkflowOptions, workflow interface{}, args ...interface{}) (WorkflowRun, error)
	DescribeWorkflow(ctx context.Context, workflowID, runID string) (*WorkflowExecutionDescription, error)
	GetWorkflowHistory(ctx context.Context, workflowID, runID string, isLongPoll bool, filterType enumspb.HistoryEventFilterType) HistoryEventIterator
	TerminateWorkflow(ctx context.Context, workflowID, runID string, reason string, details ...interface{}) error
}
