package reference_workflows

import (
	"github.com/andrewmysliuk/jobhound_core/internal/platform/temporalopts"
	reference_activities "github.com/andrewmysliuk/jobhound_core/internal/reference/workflows/activities"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// ReferenceWorkflowName is the registered workflow type for the v0 demo.
const ReferenceWorkflowName = "ReferenceDemoWorkflow"

// ReferenceActivityName is the registered activity type for the v0 demo.
const ReferenceActivityName = "ReferenceGreetActivity"

// Register attaches the reference workflow and activity to the worker (per-module registration, omg-api style).
func Register(w worker.Worker) {
	w.RegisterWorkflowWithOptions(ReferenceDemoWorkflow, workflow.RegisterOptions{
		Name: ReferenceWorkflowName,
	})
	w.RegisterActivityWithOptions(reference_activities.ReferenceGreetActivity, activity.RegisterOptions{
		Name: ReferenceActivityName,
	})
}

// ReferenceDemoWorkflow runs the v0 demo: one activity call, string in/out.
// Activity options use shared conservative defaults (see temporalopts).
func ReferenceDemoWorkflow(ctx workflow.Context, name string) (string, error) {
	ctx = workflow.WithActivityOptions(ctx, temporalopts.DefaultActivityOptions())

	var greeting string
	if err := workflow.ExecuteActivity(ctx, reference_activities.ReferenceGreetActivity, name).Get(ctx, &greeting); err != nil {
		return "", err
	}
	return "demo: " + greeting, nil
}
