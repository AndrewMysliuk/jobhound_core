package enrich_workflows

import (
	"context"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"

	enrichimpl "github.com/andrewmysliuk/jobhound_core/internal/enrich/impl"
	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	enrich_activities "github.com/andrewmysliuk/jobhound_core/internal/enrich/workflows/activities"
)

// Registrar is the worker surface this module registers onto.
// *testsuite.TestWorkflowEnvironment implements it, as does worker.Worker.
type Registrar interface {
	RegisterWorkflow(w interface{})
	RegisterActivityWithOptions(a interface{}, options activity.RegisterOptions)
}

// Deps is what EnrichBatchWorkflow activities need.
// When Enrich or Rescore is nil, only a no-op ListMissing is registered so the parent can always call it.
type Deps struct {
	Enrich  *enrichimpl.Service
	Rescore enrich_activities.Rescorer
	Log     zerolog.Logger
}

// New registers EnrichBatchWorkflow and its activities. It calls RegisterWorkflow.
// With Enrich disabled it still registers ListMissing as an empty-list stub.
func New(w Registrar, deps Deps) {
	if w == nil {
		return
	}
	if deps.Enrich == nil || deps.Rescore == nil {
		w.RegisterActivityWithOptions(listMissingDisabled, activity.RegisterOptions{
			Name: enrich_activities.ListMissingActivityName,
		})
		return
	}
	acts := &enrich_activities.Activities{
		Enrich:  deps.Enrich,
		Rescore: deps.Rescore,
		Log:     deps.Log,
	}
	w.RegisterActivityWithOptions(acts.ListMissing, activity.RegisterOptions{
		Name: enrich_activities.ListMissingActivityName,
	})
	w.RegisterActivityWithOptions(acts.SubmitBatch, activity.RegisterOptions{
		Name: enrich_activities.SubmitBatchActivityName,
	})
	w.RegisterActivityWithOptions(acts.PollBatch, activity.RegisterOptions{
		Name: enrich_activities.PollBatchActivityName,
	})
	w.RegisterActivityWithOptions(acts.StoreVerdicts, activity.RegisterOptions{
		Name: enrich_activities.StoreVerdictsActivityName,
	})
	w.RegisterActivityWithOptions(acts.RescoreJobs, activity.RegisterOptions{
		Name: enrich_activities.RescoreJobsActivityName,
	})
	w.RegisterWorkflow(EnrichBatchWorkflow)
}

func listMissingDisabled(context.Context, schema.ListMissingInput) (schema.ListMissingResult, error) {
	return schema.ListMissingResult{}, nil
}

var _ Registrar = (worker.Worker)(nil)
