package manual_workflows

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	collectorsschema "github.com/andrewmysliuk/jobhound_core/internal/collectors/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	ingest_workflows "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/temporalopts"
	"github.com/google/uuid"
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
		ctxPipe := workflow.WithActivityOptions(ctx, temporalopts.PipelinePersistActivityOptions())
		in2 := pipelineschema.PersistPipelineStage2Input{
			PipelineRunID:      runID,
			SlotID:             in.SlotID,
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
		passed := s2out.AfterKeywordsCount
		rejected := s2out.AfterBroadCount - passed
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

type ingestChildSpec struct {
	source string
	query  string
}

func expandIngestChildren(sourceIDs []string) []ingestChildSpec {
	var children []ingestChildSpec
	for _, src := range sourceIDs {
		queries := collectorsschema.Queries(src)
		if len(queries) == 0 {
			children = append(children, ingestChildSpec{source: src, query: ""})
			continue
		}
		for _, q := range queries {
			children = append(children, ingestChildSpec{source: src, query: q})
		}
	}
	return children
}

func ingestQueryWorkflowSegment(query string) string {
	q := ingest.NormalizeSourceID(query)
	if q == "" {
		return "catalog"
	}
	return sanitizeWorkflowIDSegment(q)
}

func runParallelIngest(ctx workflow.Context, agg *manualschema.ManualSlotRunAggregate, in manualschema.ManualSlotRunWorkflowInput, explicitRefresh bool) {
	info := workflow.GetInfo(ctx)
	parentID := info.WorkflowExecution.ID
	children := expandIngestChildren(in.SourceIDs)
	var parallel, serial []ingestChildSpec
	for _, ch := range children {
		if ch.source == builtin.SourceName {
			serial = append(serial, ch)
			continue
		}
		parallel = append(parallel, ch)
	}

	agg.Ingest = make(map[string]ingestschema.IngestSourceOutput, len(in.SourceIDs))
	var errParts []string
	collect := func(ch ingestChildSpec, fut workflow.ChildWorkflowFuture) {
		var out ingestschema.IngestSourceOutput
		if err := fut.Get(ctx, &out); err != nil {
			workflow.GetLogger(ctx).Error("ingest child workflow failed",
				logging.FieldWorkflow, manualschema.ManualSlotRunWorkflowName,
				logging.FieldSlotID, in.SlotID.String(),
				logging.FieldSourceID, ch.source,
				"search_query", ch.query,
				"error", err,
			)
			errParts = append(errParts, fmt.Sprintf("ingest %s (%s): %v", ch.source, ingestQueryWorkflowSegment(ch.query), err))
			return
		}
		prev := agg.Ingest[ch.source]
		prev.JobsWritten += out.JobsWritten
		prev.JobsSkipped += out.JobsSkipped
		prev.JobsFilteredOut += out.JobsFilteredOut
		prev.UsedIncremental = prev.UsedIncremental || out.UsedIncremental
		prev.WatermarkAdvanced = prev.WatermarkAdvanced || out.WatermarkAdvanced
		agg.Ingest[ch.source] = prev
	}

	futs := make([]workflow.ChildWorkflowFuture, len(parallel))
	for i, ch := range parallel {
		futs[i] = startIngestChild(ctx, parentID, in.SlotID, ch, explicitRefresh)
	}
	for i, ch := range parallel {
		collect(ch, futs[i])
	}
	for _, ch := range serial {
		collect(ch, startIngestChild(ctx, parentID, in.SlotID, ch, explicitRefresh))
	}

	if len(errParts) > 0 {
		if agg.ErrorSummary != "" {
			agg.ErrorSummary += "; "
		}
		agg.ErrorSummary += strings.Join(errParts, "; ")
	}
}

func startIngestChild(ctx workflow.Context, parentID string, slotID uuid.UUID, ch ingestChildSpec, explicitRefresh bool) workflow.ChildWorkflowFuture {
	srcSeg := sanitizeWorkflowIDSegment(ch.source)
	querySeg := ingestQueryWorkflowSegment(ch.query)
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:          fmt.Sprintf("%s-ingest-%s-%s", parentID, srcSeg, querySeg),
		WorkflowRunTimeout:  25 * time.Minute,
		WorkflowTaskTimeout: time.Minute,
	})
	return workflow.ExecuteChildWorkflow(childCtx, ingest_workflows.IngestSourceWorkflowName, ingestschema.IngestSourceInput{
		SlotID:          slotID,
		SourceID:        ch.source,
		ExplicitRefresh: explicitRefresh,
		SlotSearchQuery: ch.query,
	})
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
