package reference_workflows

import (
	"testing"

	reference_activities "github.com/andrewmysliuk/jobhound_core/internal/reference/workflows/activities"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestReferenceDemoWorkflow_inMemory(t *testing.T) {
	t.Parallel()

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ReferenceDemoWorkflow)
	env.RegisterActivity(reference_activities.ReferenceGreetActivity)

	env.ExecuteWorkflow(ReferenceDemoWorkflow, "Temporal")
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result string
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "demo: Hello, Temporal!", result)
}

func TestReferenceDemoWorkflow_inMemory_emptyNameUsesDefault(t *testing.T) {
	t.Parallel()

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ReferenceDemoWorkflow)
	env.RegisterActivity(reference_activities.ReferenceGreetActivity)

	env.ExecuteWorkflow(ReferenceDemoWorkflow, "")
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result string
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "demo: Hello, world!", result)
}
