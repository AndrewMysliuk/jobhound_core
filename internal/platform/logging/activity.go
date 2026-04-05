package logging

import (
	"context"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"
)

// LoggerWithActivity returns a child logger with handler=activityName and, when ctx is inside a Temporal activity,
// workflow_id and run_id from activity.GetInfo. Non-activity contexts (e.g. tests) are handled without panicking.
func LoggerWithActivity(ctx context.Context, base zerolog.Logger, activityName string) zerolog.Logger {
	var wid, rid string
	func() {
		defer func() { recover() }()
		info := activity.GetInfo(ctx)
		wid = info.WorkflowExecution.ID
		rid = info.WorkflowExecution.RunID
	}()

	b := base.With().Str(FieldHandler, activityName)
	if wid != "" {
		b = b.Str(FieldWorkflowID, wid)
	}
	if rid != "" {
		b = b.Str(FieldRunID, rid)
	}
	return b.Logger()
}
