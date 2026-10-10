package scoring_workflows

import (
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
	scoring_activities "github.com/andrewmysliuk/jobhound_core/internal/scoring/workflows/activities"
)

// Registrar is the worker surface this module registers onto.
// *testsuite.TestWorkflowEnvironment implements it, as does worker.Worker.
type Registrar interface {
	RegisterWorkflow(w interface{})
	RegisterActivityWithOptions(a interface{}, options activity.RegisterOptions)
}

// Deps is the set of stores a profile run reads. The workflow payload does not carry the YAML.
type Deps struct {
	Profiles profiles.Store
	Runs     *storage.Repository
	Jobs     jobs.JobRepository
	Log      zerolog.Logger
}

// New registers ProfileRunWorkflow and its activities. It calls RegisterWorkflow.
func New(w Registrar, deps Deps) {
	if w == nil || deps.Profiles == nil || deps.Runs == nil || deps.Jobs == nil {
		return
	}
	acts := &scoring_activities.Activities{
		Profiles: deps.Profiles,
		Runs:     deps.Runs,
		Jobs:     deps.Jobs,
		Log:      deps.Log,
	}
	w.RegisterActivityWithOptions(acts.LoadProfile, activity.RegisterOptions{
		Name: scoring_activities.LoadProfileActivityName,
	})
	w.RegisterActivityWithOptions(acts.ScoreProfile, activity.RegisterOptions{
		Name: scoring_activities.ScoreProfileActivityName,
	})
	w.RegisterActivityWithOptions(acts.SetRunCounters, activity.RegisterOptions{
		Name: scoring_activities.SetRunCountersActivityName,
	})
	w.RegisterActivityWithOptions(acts.MarkRunFinished, activity.RegisterOptions{
		Name: scoring_activities.MarkRunFinishedActivityName,
	})
	w.RegisterWorkflow(ProfileRunWorkflow)
}

var _ Registrar = (worker.Worker)(nil)
