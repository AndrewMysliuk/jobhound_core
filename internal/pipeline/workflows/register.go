package pipeline_workflows

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/llm"
	pipeline_activities "github.com/andrewmysliuk/jobhound_core/internal/pipeline/workflows/activities"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

// RunPipelineStagesActivityName is the registered activity name for stage 1–3 orchestration.
const RunPipelineStagesActivityName = "RunPipelineStagesActivity"

// ActivitiesDeps configures pipeline stage activities (Temporal worker wire-up).
type ActivitiesDeps struct {
	Clock  func() time.Time
	Scorer llm.Scorer
}

// RegisterActivities registers pipeline stage activities on the worker.
func RegisterActivities(w worker.Worker, deps ActivitiesDeps) {
	acts := &pipeline_activities.Activities{
		Clock:  deps.Clock,
		Scorer: deps.Scorer,
	}
	w.RegisterActivityWithOptions(acts.RunPipelineStages, activity.RegisterOptions{
		Name: RunPipelineStagesActivityName,
	})
}
