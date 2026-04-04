package impl

import (
	"errors"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// workflowExecutionRunning reports whether DescribeWorkflow shows a RUNNING execution.
// NotFound → false, nil error (no active workflow with that id).
func workflowExecutionRunning(desc *client.WorkflowExecutionDescription, err error) (bool, error) {
	if err != nil {
		var nf *serviceerror.NotFound
		if errors.As(err, &nf) {
			return false, nil
		}
		return false, err
	}
	return desc.Status == enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING, nil
}

// stageFullFromDescribe maps Temporal workflow execution state to the public API stage card (§2.3).
func stageFullFromDescribe(desc *client.WorkflowExecutionDescription, err error, failedCode, failedMessage string) schema.StageFull {
	if err != nil {
		var nf *serviceerror.NotFound
		if errors.As(err, &nf) {
			return schema.StageFull{State: schema.StageStateIdle}
		}
		return schema.StageFull{State: schema.StageStateIdle}
	}
	switch desc.Status {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:
		st := desc.WorkflowStartTime
		return schema.StageFull{State: schema.StageStateRunning, StartedAt: &st}
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		st := desc.WorkflowStartTime
		return schema.StageFull{
			State:      schema.StageStateSucceeded,
			StartedAt:  &st,
			FinishedAt: desc.WorkflowCloseTime,
		}
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED,
		enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		st := desc.WorkflowStartTime
		return schema.StageFull{
			State:      schema.StageStateFailed,
			StartedAt:  &st,
			FinishedAt: desc.WorkflowCloseTime,
			Error: &schema.StageError{
				Code:    failedCode,
				Message: failedMessage,
			},
		}
	default:
		return schema.StageFull{State: schema.StageStateIdle}
	}
}

func stage1FromDescribe(desc *client.WorkflowExecutionDescription, err error) schema.StageFull {
	return stageFullFromDescribe(desc, err, "ingest_failed", "ingest workflow did not complete successfully")
}

func stage2FromDescribe(desc *client.WorkflowExecutionDescription, err error) schema.StageFull {
	return stageFullFromDescribe(desc, err, "stage2_failed", "stage 2 workflow did not complete successfully")
}

func stage3FromDescribe(desc *client.WorkflowExecutionDescription, err error) schema.StageFull {
	return stageFullFromDescribe(desc, err, "stage3_failed", "stage 3 workflow did not complete successfully")
}
