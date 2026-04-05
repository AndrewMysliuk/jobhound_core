package impl

import (
	"context"
	"errors"
	"strings"

	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	manualworkflows "github.com/andrewmysliuk/jobhound_core/internal/manual/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	slotworkflows "github.com/andrewmysliuk/jobhound_core/internal/slots/workflows"
	"github.com/google/uuid"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

func stage2WorkflowID(slotID uuid.UUID) string {
	return "pubapi-slot-stage2-" + slotID.String()
}

func stage3WorkflowID(slotID uuid.UUID) string {
	return "pubapi-slot-stage3-" + slotID.String()
}

// RunStage2 implements [slots.API.RunStage2].
// Concurrency: fixed Temporal workflow id per slot and stage; DescribeWorkflow before ExecuteWorkflow;
// ALLOW_DUPLICATE allows a new run after the previous closed. If ExecuteWorkflow races, AlreadyStarted maps to 409.
func (s *Service) RunStage2(ctx context.Context, slotID string, include, exclude []string) (*schema.StageRunAcceptedResponse, error) {
	log := s.methodLog(ctx, "RunStage2")
	log.Debug().Msg("run stage 2")
	if s.Runs == nil {
		return nil, errors.New("slots service: pipeline runs repository is required for stage 2")
	}
	u, err := uuid.Parse(strings.TrimSpace(slotID))
	if err != nil {
		return nil, slots.ErrNotFound
	}
	if _, err := s.Repo.GetByID(ctx, u.String()); err != nil {
		return nil, err
	}
	wid := stage2WorkflowID(u)
	running, err := slotworkflows.WorkflowExecutionRunning(s.Temporal.DescribeWorkflow(ctx, wid, ""))
	if err != nil {
		return nil, err
	}
	if running {
		return nil, slots.ErrStageAlreadyRunning
	}
	if _, err := s.Runs.InvalidateStage2And3SnapshotsForSlot(ctx, u); err != nil {
		return nil, err
	}
	in := manualschema.ManualSlotRunWorkflowInput{
		SlotID:       u,
		Kind:         manualschema.RunKindPipelineStage2,
		KeywordRules: pipeline.KeywordRules{Include: append([]string(nil), include...), Exclude: append([]string(nil), exclude...)},
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	_, err = s.Temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    wid,
		TaskQueue:             s.TaskQueue,
		WorkflowRunTimeout:    manualworkflows.DefaultManualSlotRunWorkflowTimeout,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, manualschema.ManualSlotRunWorkflowName, in)
	if err != nil {
		if temporal.IsWorkflowExecutionAlreadyStartedError(err) {
			return nil, slots.ErrStageAlreadyRunning
		}
		return nil, err
	}
	return &schema.StageRunAcceptedResponse{SlotID: u.String(), Stage: 2}, nil
}

// RunStage3 implements [slots.API.RunStage3]. maxJobs must already be validated (1–100) by the handler.
func (s *Service) RunStage3(ctx context.Context, slotID string, maxJobs int) (*schema.StageRunAcceptedResponse, error) {
	log := s.methodLog(ctx, "RunStage3")
	log.Debug().Msg("run stage 3")
	if s.Runs == nil || s.Profiles == nil {
		return nil, errors.New("slots service: pipeline runs and profile are required for stage 3")
	}
	u, err := uuid.Parse(strings.TrimSpace(slotID))
	if err != nil {
		return nil, slots.ErrNotFound
	}
	if _, err := s.Repo.GetByID(ctx, u.String()); err != nil {
		return nil, err
	}
	wid := stage3WorkflowID(u)
	running, err := slotworkflows.WorkflowExecutionRunning(s.Temporal.DescribeWorkflow(ctx, wid, ""))
	if err != nil {
		return nil, err
	}
	if running {
		return nil, slots.ErrStageAlreadyRunning
	}
	runID, ok, err := s.Runs.LatestPipelineRunIDForSlot(ctx, u)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, slots.ErrNoPipelineRun
	}
	profileText, err := s.Profiles.GetText(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(profileText) == "" {
		return nil, slots.ErrProfileRequired
	}
	if _, err := s.Runs.InvalidateStage3SnapshotsForSlot(ctx, u); err != nil {
		return nil, err
	}
	rid := runID
	in := manualschema.ManualSlotRunWorkflowInput{
		SlotID:        u,
		Kind:          manualschema.RunKindPipelineStage3,
		Profile:       profileText,
		PipelineRunID: &rid,
		Stage3MaxJobs: maxJobs,
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	_, err = s.Temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    wid,
		TaskQueue:             s.TaskQueue,
		WorkflowRunTimeout:    manualworkflows.DefaultManualSlotRunWorkflowTimeout,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, manualschema.ManualSlotRunWorkflowName, in)
	if err != nil {
		if temporal.IsWorkflowExecutionAlreadyStartedError(err) {
			return nil, slots.ErrStageAlreadyRunning
		}
		return nil, err
	}
	return &schema.StageRunAcceptedResponse{SlotID: u.String(), Stage: 3}, nil
}
