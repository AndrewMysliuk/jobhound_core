package manual_workflows

import (
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	manual_activities "github.com/andrewmysliuk/jobhound_core/internal/manual/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// WorkerDeps configures manual slot workflows and their supporting activities.
type WorkerDeps struct {
	Runs pipeline.PipelineRunRepository
	Jobs jobs.JobRepository
	Log  zerolog.Logger
}

// Register registers ManualSlotRunWorkflow and DB helper activities when Runs and Jobs are configured.
func Register(w worker.Worker, deps WorkerDeps) {
	if w == nil || deps.Runs == nil || deps.Jobs == nil {
		return
	}
	acts := &manual_activities.Activities{Runs: deps.Runs, Jobs: deps.Jobs, Log: deps.Log}
	w.RegisterActivityWithOptions(acts.CreatePipelineRun, activity.RegisterOptions{
		Name: manualschema.CreatePipelineRunActivityName,
	})
	w.RegisterActivityWithOptions(acts.ListSlotJobsPassedStage1, activity.RegisterOptions{
		Name: manualschema.ListSlotJobsPassedStage1ActivityName,
	})
	w.RegisterWorkflowWithOptions(ManualSlotRunWorkflow, workflow.RegisterOptions{
		Name: manualschema.ManualSlotRunWorkflowName,
	})
}
