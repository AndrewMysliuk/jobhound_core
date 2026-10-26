package companies_workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	companies_activities "github.com/andrewmysliuk/jobhound_core/internal/companies/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

// PollCompaniesWorkflowName is the registered workflow type.
const PollCompaniesWorkflowName = "PollCompaniesWorkflow"

// PollCompaniesWorkflow polls due RESOLVED boards with per-ATS concurrency and a per-run cap.
// Workflow id is poll-companies-{run_id}. A failed poll does not fail the workflow.
func PollCompaniesWorkflow(ctx workflow.Context, in schema.PollCompaniesPayload) (schema.PollCompaniesResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("poll companies start",
		logging.FieldWorkflow, PollCompaniesWorkflowName,
		"run_id", in.RunID,
		"failure_stage", schema.FailureStagePoll.String(),
	)
	listCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})
	var due schema.ListDuePollableResult
	if err := workflow.ExecuteActivity(listCtx, companies_activities.ListDuePollableActivityName, schema.ListDuePollableInput{}).Get(ctx, &due); err != nil {
		logger.Error("list due pollable failed",
			"failure_stage", schema.FailureStagePoll.String(),
			"error", err.Error(),
		)
		return schema.PollCompaniesResult{}, err
	}
	if len(due.Targets) == 0 {
		return schema.PollCompaniesResult{}, nil
	}

	concurrency := due.ATSConcurrency
	if concurrency < 1 {
		concurrency = 5
	}

	pollCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: companies_activities.PollCompanyMaxAttempts,
		},
	})
	recordCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	type atsLane struct {
		targets []schema.PollTarget
		sem     workflow.Semaphore
	}
	lanes := map[string]*atsLane{}
	for _, t := range due.Targets {
		ats := string(t.ATS)
		lane, ok := lanes[ats]
		if !ok {
			lane = &atsLane{sem: workflow.NewSemaphore(ctx, int64(concurrency))}
			lanes[ats] = lane
		}
		lane.targets = append(lane.targets, t)
	}

	var (
		out schema.PollCompaniesResult
		mu  = workflow.NewMutex(ctx)
	)
	wg := workflow.NewWaitGroup(ctx)
	for ats, lane := range lanes {
		lane := lane
		ats := ats
		for _, target := range lane.targets {
			target := target
			wg.Add(1)
			workflow.GoNamed(ctx, "poll-"+ats+"-"+target.CompanyKey, func(gctx workflow.Context) {
				defer wg.Done()
				if err := lane.sem.Acquire(gctx, 1); err != nil {
					return
				}
				defer lane.sem.Release(1)

				var pollOut schema.PollCompanyResult
				err := workflow.ExecuteActivity(pollCtx, companies_activities.PollCompanyActivityName, schema.PollCompanyInput{
					CompanyKey: target.CompanyKey,
				}).Get(gctx, &pollOut)
				if err != nil {
					if IsBusinessSkip(err) {
						logger.Info("poll skipped",
							"failure_stage", schema.FailureStagePoll.String(),
							"company_key", target.CompanyKey,
							"error", err.Error(),
						)
						return
					}
					logger.Error("poll failed after retries",
						"failure_stage", schema.FailureStagePoll.String(),
						"company_key", target.CompanyKey,
						"error", err.Error(),
					)
					_ = workflow.ExecuteActivity(recordCtx, companies_activities.RecordPollFailureActivityName, schema.RecordPollFailureInput{
						CompanyKey: target.CompanyKey,
					}).Get(gctx, nil)
					if lockErr := mu.Lock(gctx); lockErr == nil {
						out.PollFailures++
						mu.Unlock()
					}
					return
				}
				if pollOut.Skipped {
					return
				}
				delta := ATSCountersFromPoll(pollOut)
				if lockErr := mu.Lock(gctx); lockErr == nil {
					out.CompaniesPolled += delta.CompaniesPolled
					out.PollsNotModified += delta.PollsNotModified
					out.PollsPartial += delta.PollsPartial
					out.PollFailures += delta.PollFailures
					out.JobsUpsertedATS += delta.JobsUpsertedATS
					out.JobsClosed += delta.JobsClosed
					mu.Unlock()
				}
			})
		}
	}
	wg.Wait(ctx)
	logger.Info("poll companies done",
		"companies_polled", out.CompaniesPolled,
		"poll_failures", out.PollFailures,
		"jobs_upserted_ats", out.JobsUpsertedATS,
		"jobs_closed", out.JobsClosed,
	)
	return out, nil
}
