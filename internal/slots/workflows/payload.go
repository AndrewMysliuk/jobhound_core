package workflows

import (
	"context"
	"encoding/json"
	"fmt"

	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
)

func describeShowsStageExecution(desc *client.WorkflowExecutionDescription) bool {
	if desc == nil {
		return false
	}
	switch desc.Status {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING,
		enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		enumspb.WORKFLOW_EXECUTION_STATUS_FAILED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED,
		enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return true
	default:
		return false
	}
}

// StagePayloadFromTemporal returns JSON for the last workflow run input in HTTP request shape for the given stage (1–3), or nil.
func StagePayloadFromTemporal(ctx context.Context, tc slots.WorkflowTemporal, workflowID string, desc *client.WorkflowExecutionDescription, describeErr error, stage int) *json.RawMessage {
	if describeErr != nil || !describeShowsStageExecution(desc) {
		return nil
	}
	runID := desc.WorkflowExecution.RunID
	in, ok := decodeManualInputFromHistory(ctx, tc, workflowID, runID)
	if !ok {
		return nil
	}
	b, err := publicStagePayloadBytes(stage, in)
	if err != nil {
		return nil
	}
	raw := json.RawMessage(b)
	return &raw
}

func decodeManualInputFromHistory(ctx context.Context, tc slots.WorkflowTemporal, workflowID, runID string) (manualschema.ManualSlotRunWorkflowInput, bool) {
	iter := tc.GetWorkflowHistory(ctx, workflowID, runID, false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	dc := converter.GetDefaultDataConverter()
	for iter.HasNext() {
		ev, err := iter.Next()
		if err != nil {
			return manualschema.ManualSlotRunWorkflowInput{}, false
		}
		if ev.GetEventType() != enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED {
			continue
		}
		attr := ev.GetWorkflowExecutionStartedEventAttributes()
		if attr == nil {
			continue
		}
		var in manualschema.ManualSlotRunWorkflowInput
		if err := dc.FromPayloads(attr.GetInput(), &in); err != nil {
			return manualschema.ManualSlotRunWorkflowInput{}, false
		}
		return in, true
	}
	return manualschema.ManualSlotRunWorkflowInput{}, false
}

func publicStagePayloadBytes(stage int, in manualschema.ManualSlotRunWorkflowInput) ([]byte, error) {
	switch stage {
	case 1:
		return json.Marshal(struct {
			SearchQuery     string   `json:"search_query"`
			SourceIDs       []string `json:"source_ids"`
			ExplicitRefresh bool     `json:"explicit_refresh"`
		}{
			SearchQuery:     in.SlotSearchQuery,
			SourceIDs:       append([]string(nil), in.SourceIDs...),
			ExplicitRefresh: in.ExplicitRefresh,
		})
	case 2:
		return json.Marshal(struct {
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		}{
			Include: append([]string(nil), in.KeywordRules.Include...),
			Exclude: append([]string(nil), in.KeywordRules.Exclude...),
		})
	case 3:
		return json.Marshal(struct {
			MaxJobs int `json:"max_jobs"`
		}{MaxJobs: in.Stage3MaxJobs})
	default:
		return nil, fmt.Errorf("workflows: invalid stage %d", stage)
	}
}
