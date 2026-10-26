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
const DefaultPollInterval = 30 * time.Second

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
			stored, err := enrichOneChunk(ctx, actCtx, pollCtx, storeCtx, in, page.JobIDs, chunk)
			if err != nil {
				return err
			}
			totalStored += stored
		}
	} else {
		for offset := 0; offset < len(in.JobIDs); offset += schema.MaxJobsPerSubmit {
			end := offset + schema.MaxJobsPerSubmit
			if end > len(in.JobIDs) {
				end = len(in.JobIDs)
			}
			chunk++
			stored, err := enrichOneChunk(ctx, actCtx, pollCtx, storeCtx, in, in.JobIDs[offset:end], chunk)
			if err != nil {
				return err
			}
			totalStored += stored
		}
	}

	logger.Info("enrich batch done", "chunks", chunk, "stored", totalStored)
	return nil
}

func enrichOneChunk(
	ctx, actCtx, pollCtx, storeCtx workflow.Context,
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

	var poll schema.PollBatchResult
	for {
		if err := workflow.ExecuteActivity(pollCtx, enrich_activities.PollBatchActivityName, schema.PollBatchInput{
			BatchID:     submitted.BatchID,
			CustomToJob: submitted.CustomToJob,
		}).Get(ctx, &poll); err != nil {
			logger.Error("poll batch failed", "chunk", chunk, "batch_id", submitted.BatchID, "error", err.Error())
			return 0, err
		}
		if poll.Ended {
			break
		}
		if err := workflow.Sleep(ctx, DefaultPollInterval); err != nil {
			return 0, err
		}
	}

	var stored schema.StoreVerdictsResult
	if err := workflow.ExecuteActivity(storeCtx, enrich_activities.StoreVerdictsActivityName, schema.StoreVerdictsInput{
		Texts:         poll.Texts,
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
