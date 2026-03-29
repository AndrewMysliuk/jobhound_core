//go:build integration

package reference_workflows

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

// TestReferenceDemoWorkflow_againstServer runs the v0 workflow against a real Temporal frontend.
// Requires a worker polling JOBHOUND_TEMPORAL_TASK_QUEUE (default jobhound), e.g. bin/worker with Compose up.
//
// Env: JOBHOUND_TEMPORAL_ADDRESS (see specs/003-temporal-orchestration/contracts/environment.md).
// Run: go test -tags=integration ./internal/reference/workflows/ -run TestReferenceDemoWorkflow_againstServer
func TestReferenceDemoWorkflow_againstServer(t *testing.T) {
	cfg, err := config.LoadTemporalFromEnv()
	if err != nil {
		t.Skipf("Temporal integration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := client.Dial(client.Options{
		HostPort:  cfg.Address,
		Namespace: cfg.Namespace,
	})
	require.NoError(t, err)
	defer c.Close()

	workflowID := fmt.Sprintf("integration-reference-%d", time.Now().UnixNano())
	got, err := StartReferenceDemoWorkflow(ctx, c, cfg.TaskQueue, workflowID, "integration")
	require.NoError(t, err)
	require.Equal(t, "demo: Hello, integration!", got)
}
