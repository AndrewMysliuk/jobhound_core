package pipeline_workflows

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/llm"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipeline_activities "github.com/andrewmysliuk/jobhound_core/internal/pipeline/workflows/activities"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

// RunPipelineStagesActivityName is the registered activity name for stage 1–3 orchestration.
const RunPipelineStagesActivityName = "RunPipelineStagesActivity"

// RunPersistedPipelineStagesActivityName is the registered activity for 007 cap + per-run status persistence.
const RunPersistedPipelineStagesActivityName = "RunPersistedPipelineStagesActivity"

// ActivitiesDeps configures pipeline stage activities (Temporal worker wire-up).
type ActivitiesDeps struct {
	Clock  func() time.Time
	Scorer llm.Scorer
	// RunRepo and JobsRepo enable RunPersistedPipelineStages when both are non-nil (Postgres via cmd/worker).
	RunRepo  pipeline.PipelineRunRepository
	JobsRepo jobs.JobRepository
	// Stage3MaxJobsPerRun is the 007 cap N (from config); zero uses default constant in selection helper.
	Stage3MaxJobsPerRun int
}

// RegisterActivities registers pipeline stage activities on the worker.
func RegisterActivities(w worker.Worker, deps ActivitiesDeps) {
	acts := &pipeline_activities.Activities{
		Clock:               deps.Clock,
		Scorer:              deps.Scorer,
		Runs:                deps.RunRepo,
		Jobs:                deps.JobsRepo,
		Stage3MaxJobsPerRun: deps.Stage3MaxJobsPerRun,
	}
	w.RegisterActivityWithOptions(acts.RunPipelineStages, activity.RegisterOptions{
		Name: RunPipelineStagesActivityName,
	})
	if deps.RunRepo != nil && deps.JobsRepo != nil {
		w.RegisterActivityWithOptions(acts.RunPersistedPipelineStages, activity.RegisterOptions{
			Name: RunPersistedPipelineStagesActivityName,
		})
	}
}
