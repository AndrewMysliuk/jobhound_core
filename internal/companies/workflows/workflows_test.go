package companies_workflows

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	companies_activities "github.com/andrewmysliuk/jobhound_core/internal/companies/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

func TestResolveAndPollWorkflowIDs(t *testing.T) {
	if got := ResolveWorkflowID("acme.com"); got != "resolve-acme.com" {
		t.Fatalf("resolve id = %q", got)
	}
	if got := PollWorkflowIDFromRun(42); got != "poll-companies-42" {
		t.Fatalf("poll id = %q", got)
	}
}

func TestPollCompaniesWorkflow_aggregatesCounters(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetTestTimeout(time.Minute)
	New(env, Deps{Log: logging.Nop()})

	env.OnActivity(companies_activities.ListDuePollableActivityName, mock.Anything, mock.Anything).
		Return(schema.ListDuePollableResult{
			Targets: []schema.PollTarget{
				{CompanyKey: "acme", ATS: schema.ATSGreenhouse},
				{CompanyKey: "beta", ATS: schema.ATSLever},
			},
			ATSConcurrency: 5,
		}, nil)
	env.OnActivity(companies_activities.PollCompanyActivityName, mock.Anything, mock.Anything).
		Return(schema.PollCompanyResult{Outcome: schema.PollOutcomeNotModified}, nil)

	env.ExecuteWorkflow(PollCompaniesWorkflow, schema.PollCompaniesPayload{RunID: "99"})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var out schema.PollCompaniesResult
	if err := env.GetWorkflowResult(&out); err != nil {
		t.Fatal(err)
	}
	if out.CompaniesPolled != 2 || out.PollsNotModified != 2 {
		t.Fatalf("out=%+v", out)
	}
}

func TestResolveChild_idSchemeAndFailedChildDoesNotFailParent(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetTestTimeout(time.Minute)
	New(env, Deps{Log: logging.Nop()})

	wantID := ResolveWorkflowID("broken-co")
	env.OnWorkflow(ResolveCompanyWorkflow, mock.Anything, mock.Anything).Return(
		func(ctx workflow.Context, _ schema.ResolveCompanyPayload) (schema.ResolveCompanyResult, error) {
			return schema.ResolveCompanyResult{}, errors.New("resolve boom")
		},
	)

	var resolveIDs []string
	env.SetOnChildWorkflowStartedListener(func(info *workflow.Info, _ workflow.Context, _ converter.EncodedValues) {
		if info.WorkflowType.Name == ResolveCompanyWorkflowName {
			resolveIDs = append(resolveIDs, info.WorkflowExecution.ID)
		}
	})

	env.RegisterWorkflow(parentIgnoringFailedResolve)
	env.ExecuteWorkflow(parentIgnoringFailedResolve, "broken-co")
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("parent must succeed when resolve child fails: %v", err)
	}
	if len(resolveIDs) != 1 || resolveIDs[0] != wantID {
		t.Fatalf("resolve ids=%v, want [%q]", resolveIDs, wantID)
	}
}

// parentIgnoringFailedResolve mirrors ProfileRunWorkflow: abandon policy, ignore child error.
func parentIgnoringFailedResolve(ctx workflow.Context, companyKey string) error {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:            ResolveWorkflowID(companyKey),
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		ParentClosePolicy:     enumspb.PARENT_CLOSE_POLICY_ABANDON,
	})
	var out schema.ResolveCompanyResult
	err := workflow.ExecuteChildWorkflow(childCtx, ResolveCompanyWorkflow, schema.ResolveCompanyPayload{
		CompanyKey: companyKey,
	}).Get(ctx, &out)
	_ = err // failed child must not fail the parent
	return nil
}
