package temporalopts

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
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

// PipelinePersistActivityOptions returns activity options for persisted stage-2/stage-3 pipeline work,
// including LLM scoring. Parent workflows should use this when executing PersistPipelineStage2/3 activities (008).
func PipelinePersistActivityOptions() workflow.ActivityOptions {
	o := DefaultActivityOptions()
	o.StartToCloseTimeout = 5 * time.Minute
	o.ScheduleToCloseTimeout = 10 * time.Minute
	return o
}

// DefaultWorkerOptions returns baseline [worker.Options] for cmd/worker (008 registers split pipeline activities;
// activity execution timeouts for those types are set via [PipelinePersistActivityOptions] in workflow code).
func DefaultWorkerOptions() worker.Options {
	return worker.Options{}
}
