package manual_workflows

import (
	"context"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	ingest_workflows "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows"
	ingest_activities "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows/activities"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestManualSlotRunWorkflow_pipelineStage2_inMemory(t *testing.T) {
	t.Parallel()

	slotID := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ManualSlotRunWorkflow)

	env.RegisterActivityWithOptions(func(context.Context, uuid.UUID) (int64, error) {
		return 7, nil
	}, activity.RegisterOptions{Name: manualschema.CreatePipelineRunActivityName})

	env.RegisterActivityWithOptions(func(context.Context, uuid.UUID) ([]domain.Job, error) {
		return []domain.Job{{ID: "j1"}}, nil
	}, activity.RegisterOptions{Name: manualschema.ListSlotJobsPassedStage1ActivityName})

	env.RegisterActivityWithOptions(func(context.Context, pipelineschema.PersistPipelineStage2Input) (*pipelineschema.PersistPipelineStage2Output, error) {
		return &pipelineschema.PersistPipelineStage2Output{
			AfterBroad:    []domain.Job{{ID: "j1"}, {ID: "j2"}},
			AfterKeywords: []domain.Job{{ID: "j1"}},
		}, nil
	}, activity.RegisterOptions{Name: manualschema.PersistPipelineStage2ActivityName})

	env.ExecuteWorkflow(ManualSlotRunWorkflow, manualschema.ManualSlotRunWorkflowInput{
		SlotID: slotID,
		Kind:   manualschema.RunKindPipelineStage2,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var agg manualschema.ManualSlotRunAggregate
	require.NoError(t, env.GetWorkflowResult(&agg))
	require.NotEmpty(t, agg.TemporalWorkflowID)
	require.NotEmpty(t, agg.TemporalRunID)
	require.NotNil(t, agg.PipelineRunID)
	require.Equal(t, int64(7), *agg.PipelineRunID)
	require.NotNil(t, agg.Stage2)
	require.Equal(t, 1, agg.Stage2.Passed)
	require.Equal(t, 1, agg.Stage2.Rejected)
	require.Nil(t, agg.Stage3)
}

func TestManualSlotRunWorkflow_parallelIngest_inMemory(t *testing.T) {
	t.Parallel()

	slotID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ManualSlotRunWorkflow)
	env.RegisterWorkflow(ingest_workflows.IngestSourceWorkflow)

	env.RegisterActivityWithOptions(func(context.Context, ingestschema.IngestSourceInput) (*ingestschema.IngestSourceOutput, error) {
		return &ingestschema.IngestSourceOutput{JobsWritten: 2, JobsSkipped: 1}, nil
	}, activity.RegisterOptions{Name: ingest_activities.RunIngestSourceActivityName})

	env.ExecuteWorkflow(ManualSlotRunWorkflow, manualschema.ManualSlotRunWorkflowInput{
		SlotID:    slotID,
		Kind:      manualschema.RunKindIngestSources,
		SourceIDs: []string{"src_a", "src_b"},
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var agg manualschema.ManualSlotRunAggregate
	require.NoError(t, env.GetWorkflowResult(&agg))
	require.Len(t, agg.Ingest, 2)
	require.Equal(t, 2, agg.Ingest["src_a"].JobsWritten)
	require.Equal(t, 2, agg.Ingest["src_b"].JobsWritten)
}
