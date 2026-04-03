package manual_workflows

import (
	"context"
	"testing"

	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestStartManualSlotRunWorkflow_invalidInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, err := StartManualSlotRunWorkflow(ctx, nil, "jobhound", "wf-id", manualschema.ManualSlotRunWorkflowInput{
		SlotID: uuid.Nil,
		Kind:   manualschema.RunKindPipelineStage2,
	})
	require.Error(t, err)
}
