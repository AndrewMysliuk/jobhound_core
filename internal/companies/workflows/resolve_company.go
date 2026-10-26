package companies_workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	companies_activities "github.com/andrewmysliuk/jobhound_core/internal/companies/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

// ResolveCompanyWorkflowName is the registered workflow type.
const ResolveCompanyWorkflowName = "ResolveCompanyWorkflow"

// ResolveCompanyWorkflow resolves one PENDING company to ATS + slug.
// Workflow id is resolve-{company_key} (one active per company).
func ResolveCompanyWorkflow(ctx workflow.Context, in schema.ResolveCompanyPayload) (schema.ResolveCompanyResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("resolve company start",
		logging.FieldWorkflow, ResolveCompanyWorkflowName,
		"company_key", in.CompanyKey,
		"failure_stage", schema.FailureStageResolve.String(),
	)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})
	var out schema.ResolveCompanyResult
	err := workflow.ExecuteActivity(ctx, companies_activities.ResolveCompanyActivityName, in).Get(ctx, &out)
	if err != nil {
		if IsBusinessSkip(err) {
			logger.Info("resolve skipped",
				"failure_stage", schema.FailureStageResolve.String(),
				"company_key", in.CompanyKey,
				"error", err.Error(),
			)
			return schema.ResolveCompanyResult{CompanyKey: in.CompanyKey, Skipped: true}, nil
		}
		logger.Error("resolve failed",
			"failure_stage", schema.FailureStageResolve.String(),
			"company_key", in.CompanyKey,
			"error", err.Error(),
		)
		return schema.ResolveCompanyResult{}, err
	}
	return out, nil
}
