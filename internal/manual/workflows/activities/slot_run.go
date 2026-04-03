// Package manual_activities hosts Temporal activities for the manual slot parent workflow (008).
package manual_activities

import (
	"context"
	"fmt"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/google/uuid"
)

// Activities wires DB ports for manual orchestration (constructed in cmd/worker).
type Activities struct {
	Runs pipeline.PipelineRunRepository
	Jobs jobs.JobRepository
}

// CreatePipelineRun inserts pipeline_runs for the slot (007).
func (a *Activities) CreatePipelineRun(ctx context.Context, slotID uuid.UUID) (int64, error) {
	if a == nil || a.Runs == nil {
		return 0, fmt.Errorf("manual activities: CreatePipelineRun requires Runs")
	}
	if slotID == uuid.Nil {
		return 0, fmt.Errorf("manual activities: slot_id is required")
	}
	return a.Runs.CreateRun(ctx, &slotID)
}

// ListSlotJobsPassedStage1 loads the stage-2 input pool (008 §6).
func (a *Activities) ListSlotJobsPassedStage1(ctx context.Context, slotID uuid.UUID) ([]domain.Job, error) {
	if a == nil || a.Jobs == nil {
		return nil, fmt.Errorf("manual activities: ListSlotJobsPassedStage1 requires Jobs")
	}
	if slotID == uuid.Nil {
		return nil, fmt.Errorf("manual activities: slot_id is required")
	}
	return a.Jobs.ListSlotJobsPassedStage1(ctx, slotID)
}
