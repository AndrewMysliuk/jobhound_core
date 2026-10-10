package scoring_workflows

import (
	"context"
	"errors"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/scoring"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// Starter implements scoring.RunStarter.
// Workflow id is profile-run-{profile_id}. ALLOW_DUPLICATE lets the next run start after the previous one closes.
type Starter struct {
	Client    client.Client
	TaskQueue string
}

// Start executes ProfileRunWorkflow for a run row that already exists.
func (s *Starter) Start(ctx context.Context, profileID string, runID int64) error {
	if s == nil || s.Client == nil {
		return errors.New("scoring workflows: temporal client is required")
	}
	if strings.TrimSpace(s.TaskQueue) == "" {
		return errors.New("scoring workflows: task queue is required")
	}
	_, err := s.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    WorkflowID(profileID),
		TaskQueue:             s.TaskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, ProfileRunWorkflow, scoringschema.ProfileRunInput{
		ProfileID: profileID,
		RunID:     runID,
	})
	if err == nil {
		return nil
	}
	var already *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &already) {
		return scoring.ErrRunAlreadyRunning
	}
	return err
}

var _ scoring.RunStarter = (*Starter)(nil)
