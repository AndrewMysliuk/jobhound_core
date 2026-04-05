package pipeline_workflows

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/llm"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipeline_activities "github.com/andrewmysliuk/jobhound_core/internal/pipeline/workflows/activities"
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

// RunPipelineStagesActivityName is the registered activity name for stage 1–3 orchestration.
const RunPipelineStagesActivityName = "RunPipelineStagesActivity"

// ActivitiesDeps configures pipeline stage activities (Temporal worker wire-up).
type ActivitiesDeps struct {
	Clock  func() time.Time
	Scorer llm.Scorer
	// RunRepo and JobsRepo enable persisted stage-2/3 activities when both are non-nil (Postgres via cmd/worker).
	RunRepo  pipeline.PipelineRunRepository
	JobsRepo jobs.JobRepository
	// Stage3MaxJobsPerRun is the cap N (from config); zero uses default constant in selection helper.
	Stage3MaxJobsPerRun int
	Log                 zerolog.Logger
}

// RegisterActivities registers pipeline stage activities on the worker.
// Persisted stage-2/3 split activities use names from [manualschema] (008). Workflows must execute them with
// [github.com/andrewmysliuk/jobhound_core/internal/platform/temporalopts.PipelinePersistActivityOptions]
// (or stricter) so LLM calls have adequate timeouts.
func RegisterActivities(w worker.Worker, deps ActivitiesDeps) {
	acts := &pipeline_activities.Activities{
		Clock:               deps.Clock,
		Scorer:              deps.Scorer,
		Runs:                deps.RunRepo,
		Jobs:                deps.JobsRepo,
		Stage3MaxJobsPerRun: deps.Stage3MaxJobsPerRun,
		Log:                 deps.Log,
	}
	w.RegisterActivityWithOptions(acts.RunPipelineStages, activity.RegisterOptions{
		Name: RunPipelineStagesActivityName,
	})
	if deps.RunRepo != nil && deps.JobsRepo != nil {
		w.RegisterActivityWithOptions(acts.RunPersistPipelineStage2, activity.RegisterOptions{
			Name: manualschema.PersistPipelineStage2ActivityName,
		})
		w.RegisterActivityWithOptions(acts.RunPersistPipelineStage3, activity.RegisterOptions{
			Name: manualschema.PersistPipelineStage3ActivityName,
		})
	}
}
