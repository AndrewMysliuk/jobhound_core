//go:build integration

package manual_workflows

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

// TestManualSlotRunWorkflow_againstServer runs PIPELINE_STAGE2 against a real Temporal frontend and worker
// (specs/008 quality gate I.3: worker registration smoke with a running bin/worker).
// Requires bin/worker with Postgres (JOBHOUND_DATABASE_URL), same task queue/namespace as defaults (jobhound / default).
//
// Env: JOBHOUND_TEMPORAL_ADDRESS (see specs/003-temporal-orchestration/contracts/environment.md).
// Run: go test -tags=integration ./internal/manual/workflows/ -run TestManualSlotRunWorkflow_againstServer
func TestManualSlotRunWorkflow_againstServer(t *testing.T) {
	cfg, err := config.LoadTemporalFromEnv()
	if err != nil {
		t.Skipf("Temporal integration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	c, err := client.Dial(client.Options{
		HostPort:  cfg.Address,
		Namespace: cfg.Namespace,
	})
	require.NoError(t, err)
	defer c.Close()

	slotID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	workflowID := fmt.Sprintf("integration-manual-slot-%d", time.Now().UnixNano())
	got, err := StartManualSlotRunWorkflow(ctx, c, cfg.TaskQueue, workflowID, manualschema.ManualSlotRunWorkflowInput{
		SlotID: slotID,
		Kind:   manualschema.RunKindPipelineStage2,
	})
	require.NoError(t, err)
	require.Equal(t, workflowID, got.TemporalWorkflowID)
	require.NotEmpty(t, got.TemporalRunID)
	require.NotNil(t, got.PipelineRunID)
	require.NotNil(t, got.Stage2)
}
