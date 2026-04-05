package manual_workflows

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	ingest_workflows "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/temporalopts"
	"go.temporal.io/sdk/workflow"
)

// ManualSlotRunWorkflow composes parallel ingest children and persisted stage-2/stage-3 activities (008).
func ManualSlotRunWorkflow(ctx workflow.Context, in manualschema.ManualSlotRunWorkflowInput) (manualschema.ManualSlotRunAggregate, error) {
	if err := in.Validate(); err != nil {
		return manualschema.ManualSlotRunAggregate{}, err
	}

	startKV := []interface{}{
		logging.FieldWorkflow, manualschema.ManualSlotRunWorkflowName,
		logging.FieldSlotID, in.SlotID.String(),
		"run_kind", string(in.Kind),
	}
	if in.UserID != nil && strings.TrimSpace(*in.UserID) != "" {
		startKV = append(startKV, logging.FieldUserID, strings.TrimSpace(*in.UserID))
	}
	workflow.GetLogger(ctx).Info("manual slot run workflow start", startKV...)

	info := workflow.GetInfo(ctx)
	agg := manualschema.ManualSlotRunAggregate{
		TemporalWorkflowID: info.WorkflowExecution.ID,
		TemporalRunID:      info.WorkflowExecution.RunID,
	}

	explicitRefresh := in.ExplicitRefresh
	if in.Kind == manualschema.RunKindDeltaIngestThenPipeline {
		explicitRefresh = false
	}

	if in.NeedsIngest() {
		runParallelIngest(ctx, &agg, in, explicitRefresh)
	}

	var runID int64
	if in.NeedsStage2() || in.NeedsStage3() {
		if in.Kind == manualschema.RunKindPipelineStage3 {
			runID = *in.PipelineRunID
			rid := runID
			agg.PipelineRunID = &rid
		} else {
			ctxCreate := workflow.WithActivityOptions(ctx, temporalopts.DefaultActivityOptions())
			if err := workflow.ExecuteActivity(ctxCreate, manualschema.CreatePipelineRunActivityName, in.SlotID).Get(ctxCreate, &runID); err != nil {
				workflow.GetLogger(ctx).Error("CreatePipelineRun activity failed",
					logging.FieldWorkflow, manualschema.ManualSlotRunWorkflowName,
					logging.FieldSlotID, in.SlotID.String(),
					"error", err,
				)
				return agg, err
			}
			rid := runID
			agg.PipelineRunID = &rid
		}
	}

	if in.NeedsStage2() {
		ctxAct := workflow.WithActivityOptions(ctx, temporalopts.DefaultActivityOptions())
		var jobs []schema.Job
		if err := workflow.ExecuteActivity(ctxAct, manualschema.ListSlotJobsPassedStage1ActivityName, in.SlotID).Get(ctxAct, &jobs); err != nil {
			workflow.GetLogger(ctx).Error("ListSlotJobsPassedStage1 activity failed",
				logging.FieldWorkflow, manualschema.ManualSlotRunWorkflowName,
				logging.FieldSlotID, in.SlotID.String(),
				"error", err,
			)
			return agg, err
		}
		ctxPipe := workflow.WithActivityOptions(ctx, temporalopts.PipelinePersistActivityOptions())
		in2 := pipelineschema.PersistPipelineStage2Input{
			PipelineRunID:      runID,
			Jobs:               jobs,
			BroadRules:         in.BroadRules,
			KeywordRules:       in.KeywordRules,
			BroadFilterKeyHash: in.BroadFilterKeyHash,
		}
		var s2out pipelineschema.PersistPipelineStage2Output
		if err := workflow.ExecuteActivity(ctxPipe, manualschema.PersistPipelineStage2ActivityName, in2).Get(ctxPipe, &s2out); err != nil {
			workflow.GetLogger(ctx).Error("PersistPipelineStage2 activity failed",
				logging.FieldWorkflow, manualschema.ManualSlotRunWorkflowName,
				logging.FieldSlotID, in.SlotID.String(),
				logging.FieldPipelineRunID, strconv.FormatInt(runID, 10),
				"error", err,
			)
			return agg, err
		}
		passed := len(s2out.AfterKeywords)
		rejected := len(s2out.AfterBroad) - passed
		if rejected < 0 {
			rejected = 0
		}
		agg.Stage2 = &manualschema.Stage2Aggregate{Passed: passed, Rejected: rejected}
	}

	if in.NeedsStage3() {
		ctxPipe := workflow.WithActivityOptions(ctx, temporalopts.PipelinePersistActivityOptions())
		in3 := pipelineschema.PersistPipelineStage3Input{
			PipelineRunID: runID,
			Profile:       in.Profile,
			MaxJobs:       in.Stage3MaxJobs,
		}
		var s3out pipelineschema.PersistPipelineStage3Output
		if err := workflow.ExecuteActivity(ctxPipe, manualschema.PersistPipelineStage3ActivityName, in3).Get(ctxPipe, &s3out); err != nil {
			workflow.GetLogger(ctx).Error("PersistPipelineStage3 activity failed",
				logging.FieldWorkflow, manualschema.ManualSlotRunWorkflowName,
				logging.FieldSlotID, in.SlotID.String(),
				logging.FieldPipelineRunID, strconv.FormatInt(runID, 10),
				"error", err,
			)
			return agg, err
		}
		capN := pipeutils.MaxStage3JobsPerPipelineRunExecution
		var passedN, rejectedN int
		for _, sj := range s3out.Scored {
			if pipeutils.TerminalRunJobStatusFromScoredJob(sj) == pipeline.RunJobPassedStage3 {
				passedN++
			} else {
				rejectedN++
			}
		}
		agg.Stage3 = &manualschema.Stage3Aggregate{
			Scored:   len(s3out.Scored),
			Cap:      capN,
			Passed:   passedN,
			Rejected: rejectedN,
		}
	}

	return agg, nil
}

func runParallelIngest(ctx workflow.Context, agg *manualschema.ManualSlotRunAggregate, in manualschema.ManualSlotRunWorkflowInput, explicitRefresh bool) {
	info := workflow.GetInfo(ctx)
	parentID := info.WorkflowExecution.ID
	futures := make([]workflow.ChildWorkflowFuture, len(in.SourceIDs))
	for i, src := range in.SourceIDs {
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID:          fmt.Sprintf("%s-ingest-%s", parentID, sanitizeWorkflowIDSegment(src)),
			WorkflowRunTimeout:  25 * time.Minute,
			WorkflowTaskTimeout: time.Minute,
		})
		inIngest := ingestschema.IngestSourceInput{
			SlotID:          in.SlotID,
			SourceID:        src,
			ExplicitRefresh: explicitRefresh,
		}
		futures[i] = workflow.ExecuteChildWorkflow(childCtx, ingest_workflows.IngestSourceWorkflowName, inIngest)
	}

	agg.Ingest = make(map[string]ingestschema.IngestSourceOutput, len(in.SourceIDs))
	var errParts []string
	for i, src := range in.SourceIDs {
		var out ingestschema.IngestSourceOutput
		if err := futures[i].Get(ctx, &out); err != nil {
			workflow.GetLogger(ctx).Error("ingest child workflow failed",
				logging.FieldWorkflow, manualschema.ManualSlotRunWorkflowName,
				logging.FieldSlotID, in.SlotID.String(),
				logging.FieldSourceID, src,
				"error", err,
			)
			errParts = append(errParts, fmt.Sprintf("ingest %s: %v", src, err))
			continue
		}
		agg.Ingest[src] = out
	}
	if len(errParts) > 0 {
		if agg.ErrorSummary != "" {
			agg.ErrorSummary += "; "
		}
		agg.ErrorSummary += strings.Join(errParts, "; ")
	}
}

func sanitizeWorkflowIDSegment(s string) string {
	if s == "" {
		return "source"
	}
	b := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b = append(b, r)
		default:
			b = append(b, '_')
		}
	}
	return string(b)
}
