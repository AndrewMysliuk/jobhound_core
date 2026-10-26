package companies_workflows

import (
	"context"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"

	companiesimpl "github.com/andrewmysliuk/jobhound_core/internal/companies/impl"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	companies_activities "github.com/andrewmysliuk/jobhound_core/internal/companies/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
)

// Registrar is the worker surface this module registers onto.
// *testsuite.TestWorkflowEnvironment implements it, as does worker.Worker.
type Registrar interface {
	RegisterWorkflow(w interface{})
	RegisterActivityWithOptions(a interface{}, options activity.RegisterOptions)
}

// Deps is what company workflows need. When Companies is nil, stub activities
// return empty results so ProfileRunWorkflow can always call them.
type Deps struct {
	Companies *companiesimpl.Service
	Jobs      jobs.JobRepository
	Runs      *storage.Repository
	Log       zerolog.Logger
}

// New registers ResolveCompanyWorkflow, PollCompaniesWorkflow, and activities.
// It calls RegisterWorkflow for both workflows.
func New(w Registrar, deps Deps) {
	if w == nil {
		return
	}
	if deps.Companies == nil || deps.Jobs == nil || deps.Runs == nil {
		registerDisabledStubs(w)
		return
	}
	acts := &companies_activities.Activities{
		Companies: deps.Companies,
		Jobs:      deps.Jobs,
		Runs:      deps.Runs,
		Log:       deps.Log,
	}
	w.RegisterActivityWithOptions(acts.UpsertCompaniesFromJobs, activity.RegisterOptions{
		Name: companies_activities.UpsertCompaniesFromJobsActivityName,
	})
	w.RegisterActivityWithOptions(acts.ListPendingResolve, activity.RegisterOptions{
		Name: companies_activities.ListPendingResolveActivityName,
	})
	w.RegisterActivityWithOptions(acts.ResolveCompany, activity.RegisterOptions{
		Name: companies_activities.ResolveCompanyActivityName,
	})
	w.RegisterActivityWithOptions(acts.ListDuePollable, activity.RegisterOptions{
		Name: companies_activities.ListDuePollableActivityName,
	})
	w.RegisterActivityWithOptions(acts.PollCompany, activity.RegisterOptions{
		Name: companies_activities.PollCompanyActivityName,
	})
	w.RegisterActivityWithOptions(acts.RecordPollFailure, activity.RegisterOptions{
		Name: companies_activities.RecordPollFailureActivityName,
	})
	w.RegisterWorkflow(ResolveCompanyWorkflow)
	w.RegisterWorkflow(PollCompaniesWorkflow)
}

func registerDisabledStubs(w Registrar) {
	w.RegisterActivityWithOptions(upsertCompaniesDisabled, activity.RegisterOptions{
		Name: companies_activities.UpsertCompaniesFromJobsActivityName,
	})
	w.RegisterActivityWithOptions(listPendingDisabled, activity.RegisterOptions{
		Name: companies_activities.ListPendingResolveActivityName,
	})
	w.RegisterActivityWithOptions(resolveDisabled, activity.RegisterOptions{
		Name: companies_activities.ResolveCompanyActivityName,
	})
	w.RegisterActivityWithOptions(listDueDisabled, activity.RegisterOptions{
		Name: companies_activities.ListDuePollableActivityName,
	})
	w.RegisterActivityWithOptions(pollDisabled, activity.RegisterOptions{
		Name: companies_activities.PollCompanyActivityName,
	})
	w.RegisterActivityWithOptions(recordFailureDisabled, activity.RegisterOptions{
		Name: companies_activities.RecordPollFailureActivityName,
	})
	w.RegisterWorkflow(ResolveCompanyWorkflow)
	w.RegisterWorkflow(PollCompaniesWorkflow)
}

func upsertCompaniesDisabled(context.Context, schema.UpsertCompaniesFromJobsInput) (schema.UpsertCompaniesFromJobsResult, error) {
	return schema.UpsertCompaniesFromJobsResult{}, nil
}

func listPendingDisabled(context.Context, schema.ListPendingResolveInput) (schema.ListPendingResolveResult, error) {
	return schema.ListPendingResolveResult{}, nil
}

func resolveDisabled(context.Context, schema.ResolveCompanyPayload) (schema.ResolveCompanyResult, error) {
	return schema.ResolveCompanyResult{Skipped: true}, nil
}

func listDueDisabled(context.Context, schema.ListDuePollableInput) (schema.ListDuePollableResult, error) {
	return schema.ListDuePollableResult{ATSConcurrency: 5}, nil
}

func pollDisabled(context.Context, schema.PollCompanyInput) (schema.PollCompanyResult, error) {
	return schema.PollCompanyResult{Skipped: true}, nil
}

func recordFailureDisabled(context.Context, schema.RecordPollFailureInput) error {
	return nil
}

var _ Registrar = (worker.Worker)(nil)
