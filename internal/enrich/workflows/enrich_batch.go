package enrich_workflows

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	enrich_activities "github.com/andrewmysliuk/jobhound_core/internal/enrich/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EnrichBatchWorkflowName is the registered workflow type.
const EnrichBatchWorkflowName = "EnrichBatchWorkflow"

// DefaultPollInterval is how long the workflow sleeps between PollBatch checks.
// Message Batches take minutes to hours, and every poll costs history events,
// so this stays coarse on purpose.
const DefaultPollInterval = 60 * time.Second

// continueAsNewHistoryLength and continueAsNewHistorySize are the thresholds for
// rolling the run between chunks. Temporal terminates a workflow at 51200 events
// or 50MB of history; a drain spans many batches and can run for days, so the
// run is rolled well before either limit.
const (
	continueAsNewHistoryLength = 10_000
	continueAsNewHistorySize   = 10 << 20
)

// shouldContinueAsNew is true once this run's history is large enough that the
// remaining chunks belong in a fresh run.
func shouldContinueAsNew(ctx workflow.Context) bool {
	info := workflow.GetInfo(ctx)
	return info.GetCurrentHistoryLength() >= continueAsNewHistoryLength ||
		info.GetCurrentHistorySize() >= continueAsNewHistorySize
}

// EnrichBatchWorkflow submits Message Batches in Temporal-safe pages, polls each
// until it ends, stores verdicts, and rescores matches.
// Empty JobIDs drains via ListMissing pages of schema.MaxJobsPerSubmit.
// Non-empty JobIDs are processed in slices of the same size (caller-provided list).
func EnrichBatchWorkflow(ctx workflow.Context, in schema.EnrichBatchInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("enrich batch start",
		logging.FieldWorkflow, EnrichBatchWorkflowName,
		"job_count", len(in.JobIDs),
		"drain", len(in.JobIDs) == 0,
		"prompt_version", in.PromptVersion,
		"model", in.Model,
	)

	transportRetry := &temporal.RetryPolicy{
		MaximumAttempts: enrich_activities.StoreVerdictsMaxAttempts,
	}
	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         transportRetry,
	})
	pollCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         transportRetry,
	})
	fetchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         transportRetry,
	})
	storeCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: enrich_activities.StoreVerdictsMaxAttempts,
		},
	})

	chunk := 0
	totalStored := 0

	if len(in.JobIDs) == 0 {
		for {
			var page schema.ListMissingResult
			if err := workflow.ExecuteActivity(actCtx, enrich_activities.ListMissingActivityName, schema.ListMissingInput{
				PromptVersion: in.PromptVersion,
				Model:         in.Model,
				Limit:         schema.MaxJobsPerSubmit,
			}).Get(ctx, &page); err != nil {
				logger.Error("list missing failed", "chunk", chunk+1, "error", err.Error())
				return err
			}
			if in.Model == "" {
				in.Model = page.Model
			}
			if in.PromptVersion == 0 {
				in.PromptVersion = page.PromptVersion
			}
			if len(page.JobIDs) == 0 {
				break
			}
			chunk++
			stored, err := enrichOneChunk(ctx, actCtx, pollCtx, fetchCtx, storeCtx, in, page.JobIDs, chunk)
			if err != nil {
				return err
			}
			totalStored += stored
			if shouldContinueAsNew(ctx) {
				logger.Info("enrich batch continue as new",
					"chunks", chunk,
					"stored", in.StoredSoFar+totalStored,
					"history_length", workflow.GetInfo(ctx).GetCurrentHistoryLength(),
				)
				return workflow.NewContinueAsNewError(ctx, EnrichBatchWorkflow, schema.EnrichBatchInput{
					PromptVersion: in.PromptVersion,
					Model:         in.Model,
					StoredSoFar:   in.StoredSoFar + totalStored,
				})
			}
		}
	} else {
		for offset := 0; offset < len(in.JobIDs); offset += schema.MaxJobsPerSubmit {
			end := offset + schema.MaxJobsPerSubmit
			if end > len(in.JobIDs) {
				end = len(in.JobIDs)
			}
			chunk++
			stored, err := enrichOneChunk(ctx, actCtx, pollCtx, fetchCtx, storeCtx, in, in.JobIDs[offset:end], chunk)
			if err != nil {
				return err
			}
			totalStored += stored
			if end < len(in.JobIDs) && shouldContinueAsNew(ctx) {
				logger.Info("enrich batch continue as new",
					"chunks", chunk,
					"stored", in.StoredSoFar+totalStored,
					"remaining", len(in.JobIDs)-end,
					"history_length", workflow.GetInfo(ctx).GetCurrentHistoryLength(),
				)
				return workflow.NewContinueAsNewError(ctx, EnrichBatchWorkflow, schema.EnrichBatchInput{
					JobIDs:        in.JobIDs[end:],
					PromptVersion: in.PromptVersion,
					Model:         in.Model,
					StoredSoFar:   in.StoredSoFar + totalStored,
				})
			}
		}
	}

	logger.Info("enrich batch done", "chunks", chunk, "stored", in.StoredSoFar+totalStored)
	return nil
}

func enrichOneChunk(
	ctx, actCtx, pollCtx, fetchCtx, storeCtx workflow.Context,
	in schema.EnrichBatchInput,
	jobIDs []string,
	chunk int,
) (int, error) {
	logger := workflow.GetLogger(ctx)
	var submitted schema.SubmitBatchResult
	if err := workflow.ExecuteActivity(actCtx, enrich_activities.SubmitBatchActivityName, schema.EnrichBatchInput{
		JobIDs:        jobIDs,
		PromptVersion: in.PromptVersion,
		Model:         in.Model,
	}).Get(ctx, &submitted); err != nil {
		logger.Error("submit batch failed", "chunk", chunk, "error", err.Error())
		return 0, err
	}
	if submitted.BatchID == "" {
		logger.Info("enrich batch chunk skipped", "chunk", chunk, "reason", "nothing to submit")
		return 0, nil
	}

	// Poll carries the batch id only: this activity repeats for as long as the
	// batch runs and each input lands in history. Results are fetched once below.
	var poll schema.PollBatchResult
	// lastDone suppresses the progress line while the counts stand still: a
	// drain polls once a minute for hours and only movement is worth a log.
	lastDone := -1
	for {
		if err := workflow.ExecuteActivity(pollCtx, enrich_activities.PollBatchActivityName, schema.PollBatchInput{
			BatchID: submitted.BatchID,
		}).Get(ctx, &poll); err != nil {
			logger.Error("poll batch failed", "chunk", chunk, "batch_id", submitted.BatchID, "error", err.Error())
			return 0, err
		}
		if done := poll.Counts.Done(); done != lastDone {
			logger.Info("enrich batch progress",
				"chunk", chunk,
				"batch_id", submitted.BatchID,
				"status", poll.Status,
				"total", poll.Counts.Total(),
				"processing", poll.Counts.Processing,
				"succeeded", poll.Counts.Succeeded,
				"errored", poll.Counts.Errored,
				"canceled", poll.Counts.Canceled,
				"expired", poll.Counts.Expired,
			)
			lastDone = done
		}
		if poll.Ended {
			break
		}
		if err := workflow.Sleep(ctx, DefaultPollInterval); err != nil {
			return 0, err
		}
	}

	var fetched schema.FetchResultsResult
	if err := workflow.ExecuteActivity(fetchCtx, enrich_activities.FetchResultsActivityName, schema.FetchResultsInput{
		BatchID: submitted.BatchID,
		JobIDs:  submitted.SubmittedIDs,
	}).Get(ctx, &fetched); err != nil {
		logger.Error("fetch results failed", "chunk", chunk, "batch_id", submitted.BatchID, "error", err.Error())
		return 0, err
	}

	var stored schema.StoreVerdictsResult
	if err := workflow.ExecuteActivity(storeCtx, enrich_activities.StoreVerdictsActivityName, schema.StoreVerdictsInput{
		Texts:         fetched.Texts,
		PromptVersion: in.PromptVersion,
		Model:         in.Model,
	}).Get(ctx, &stored); err != nil {
		logger.Error("store verdicts failed", "chunk", chunk, "error", err.Error())
		return 0, err
	}

	if len(stored.StoredIDs) > 0 {
		if err := workflow.ExecuteActivity(storeCtx, enrich_activities.RescoreJobsActivityName, schema.RescoreJobsInput{
			JobIDs: stored.StoredIDs,
		}).Get(ctx, nil); err != nil {
			logger.Error("rescore jobs failed", "chunk", chunk, "error", err.Error())
			return 0, err
		}
	}

	logger.Info("enrich batch chunk done",
		"chunk", chunk,
		"batch_id", submitted.BatchID,
		"submitted", len(submitted.SubmittedIDs),
		"stored", len(stored.StoredIDs),
	)
	return len(stored.StoredIDs), nil
}
