package temporalopts

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DefaultActivityOptions returns conservative activity timeouts and a bounded retry policy
// for workflows that call ordinary (non-local) activities. Values match
// specs/003-temporal-orchestration/contracts/reference-workflow.md; reuse from new workflows
// so defaults stay consistent.
func DefaultActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		StartToCloseTimeout:    30 * time.Second,
		ScheduleToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
		},
	}
}
