package jobs_workflows

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

const (
	// JobRetentionScheduleID is the stable Temporal schedule id for weekly retention (UTC).
	JobRetentionScheduleID = "jobhound-job-retention"
)

// EnsureJobRetentionSchedule creates the weekly UTC retention schedule if it does not exist.
// Fires Sundays at 05:00 UTC (once per calendar week; see retention-jobs.md schedule notes).
func EnsureJobRetentionSchedule(ctx context.Context, c client.Client, taskQueue string) error {
	if c == nil || taskQueue == "" {
		return errors.New("temporal client and task queue are required")
	}
	spec := client.ScheduleSpec{
		Calendars: []client.ScheduleCalendarSpec{
			{
				Second:    []client.ScheduleRange{{Start: 0}},
				Minute:    []client.ScheduleRange{{Start: 0}},
				Hour:      []client.ScheduleRange{{Start: 5}},
				DayOfWeek: []client.ScheduleRange{{Start: 0}},
			},
		},
		TimeZoneName: "UTC",
	}
	action := &client.ScheduleWorkflowAction{
		ID:                 "job-retention-",
		Workflow:           JobRetentionWorkflowName,
		TaskQueue:          taskQueue,
		WorkflowRunTimeout: 20 * time.Minute,
	}
	_, err := c.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID:     JobRetentionScheduleID,
		Spec:   spec,
		Action: action,
	})
	if err != nil && !errors.Is(err, temporal.ErrScheduleAlreadyRunning) {
		return err
	}
	return nil
}
