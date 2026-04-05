// Package manual_activities hosts Temporal activities for the manual slot parent workflow (008).
package manual_activities

import (
	"context"
	"fmt"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Activities wires DB ports for manual orchestration (constructed in cmd/worker).
type Activities struct {
	Runs pipeline.PipelineRunRepository
	Jobs jobs.JobRepository
	Log  zerolog.Logger
}

// CreatePipelineRun inserts pipeline_runs for the slot (007).
func (a *Activities) CreatePipelineRun(ctx context.Context, slotID uuid.UUID) (int64, error) {
	if a == nil || a.Runs == nil {
		return 0, fmt.Errorf("manual activities: CreatePipelineRun requires Runs")
	}
	if slotID == uuid.Nil {
		return 0, fmt.Errorf("manual activities: slot_id is required")
	}
	ctx = logging.WithSlotID(ctx, slotID.String())
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, manualschema.CreatePipelineRunActivityName))
	log.Debug().Msg("create pipeline run")
	id, err := a.Runs.CreateRun(ctx, &slotID)
	if err != nil {
		log.Error().Err(err).Msg("create pipeline run")
		return 0, err
	}
	log.Debug().Int64(logging.FieldPipelineRunID, id).Msg("pipeline run created")
	return id, nil
}

// ListSlotJobsPassedStage1 loads the stage-2 input pool (008 §6).
func (a *Activities) ListSlotJobsPassedStage1(ctx context.Context, slotID uuid.UUID) ([]domain.Job, error) {
	if a == nil || a.Jobs == nil {
		return nil, fmt.Errorf("manual activities: ListSlotJobsPassedStage1 requires Jobs")
	}
	if slotID == uuid.Nil {
		return nil, fmt.Errorf("manual activities: slot_id is required")
	}
	ctx = logging.WithSlotID(ctx, slotID.String())
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, manualschema.ListSlotJobsPassedStage1ActivityName))
	log.Debug().Msg("list slot jobs passed stage 1")
	list, err := a.Jobs.ListSlotJobsPassedStage1(ctx, slotID)
	if err != nil {
		log.Error().Err(err).Msg("list slot jobs passed stage 1")
		return nil, err
	}
	log.Debug().Int("job_count", len(list)).Msg("list slot jobs passed stage 1 done")
	return list, nil
}
