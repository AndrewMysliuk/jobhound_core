package schema

import (
	"fmt"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/google/uuid"
)

// ManualSlotRunWorkflowInput is the parent manual orchestration payload (contracts/manual-workflow.md §4.2).
type ManualSlotRunWorkflowInput struct {
	SlotID    uuid.UUID
	UserID    *string
	Kind      RunKind
	Profile   string
	SourceIDs []string

	// ExplicitRefresh is passed to each IngestSourceWorkflow child when ingest runs, except
	// RunKindDeltaIngestThenPipeline which forces false (006 incremental path).
	ExplicitRefresh bool

	BroadRules   pipeline.BroadFilterRules
	KeywordRules pipeline.KeywordRules
	// BroadFilterKeyHash optional SHA-256 hex persisted on pipeline_runs when stage 2 runs (006).
	BroadFilterKeyHash string

	// PipelineRunID when Kind is RunKindPipelineStage3: existing run whose PASSED_STAGE_2 rows feed stage 3.
	PipelineRunID *int64
}

// Validate checks required fields for the selected run kind.
func (in ManualSlotRunWorkflowInput) Validate() error {
	if in.SlotID == uuid.Nil {
		return fmt.Errorf("manual slot run: slot_id is required")
	}
	switch in.Kind {
	case RunKindIngestSources, RunKindIngestThenPipeline, RunKindDeltaIngestThenPipeline:
		if len(in.SourceIDs) == 0 {
			return fmt.Errorf("manual slot run: source_ids required for run kind %q", in.Kind)
		}
	case RunKindPipelineStage3:
		if in.PipelineRunID == nil || *in.PipelineRunID <= 0 {
			return fmt.Errorf("manual slot run: pipeline_run_id required for PIPELINE_STAGE3")
		}
	case RunKindPipelineStage2, RunKindPipelineStage2Then3:
		// no extra fields beyond slot and rules
	case "":
		return fmt.Errorf("manual slot run: run kind is required")
	default:
		return fmt.Errorf("manual slot run: unknown run kind %q", in.Kind)
	}
	if in.NeedsStage3() && strings.TrimSpace(in.Profile) == "" {
		return fmt.Errorf("manual slot run: profile is required when stage 3 runs")
	}
	return nil
}

// NeedsIngest reports whether this run kind starts IngestSourceWorkflow children.
func (in ManualSlotRunWorkflowInput) NeedsIngest() bool {
	switch in.Kind {
	case RunKindIngestSources, RunKindIngestThenPipeline, RunKindDeltaIngestThenPipeline:
		return true
	default:
		return false
	}
}

// NeedsStage2 reports whether persisted stage 2 runs in this execution.
func (in ManualSlotRunWorkflowInput) NeedsStage2() bool {
	switch in.Kind {
	case RunKindPipelineStage2, RunKindPipelineStage2Then3,
		RunKindIngestThenPipeline, RunKindDeltaIngestThenPipeline:
		return true
	default:
		return false
	}
}

// NeedsStage3 reports whether persisted stage 3 runs in this execution.
func (in ManualSlotRunWorkflowInput) NeedsStage3() bool {
	switch in.Kind {
	case RunKindPipelineStage3, RunKindPipelineStage2Then3,
		RunKindIngestThenPipeline, RunKindDeltaIngestThenPipeline:
		return true
	default:
		return false
	}
}
