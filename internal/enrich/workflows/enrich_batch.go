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

// EnrichBatchWorkflow submits a Message Batch, polls until it ends, stores verdicts, and rescores matches.
// Activities re-read vacancy text; the payload carries only ids, prompt version, and model.
func EnrichBatchWorkflow(ctx workflow.Context, in schema.EnrichBatchInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("enrich batch start",
		logging.FieldWorkflow, EnrichBatchWorkflowName,
		"job_count", len(in.JobIDs),
		"prompt_version", in.PromptVersion,
		"model", in.Model,
	)
	if len(in.JobIDs) == 0 {
		return nil
	}

	// Transport / batch errors retry; no verdict row is written until StoreVerdicts.
	transportRetry := &temporal.RetryPolicy{
		MaximumAttempts: enrich_activities.StoreVerdictsMaxAttempts,
	}
	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         transportRetry,
	})

	var submitted schema.SubmitBatchResult
	if err := workflow.ExecuteActivity(actCtx, enrich_activities.SubmitBatchActivityName, in).Get(ctx, &submitted); err != nil {
		logger.Error("submit batch failed", "error", err.Error())
		return err
	}
	if submitted.BatchID == "" {
		logger.Info("enrich batch skipped", "reason", "nothing to submit")
		return nil
	}

	pollCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         transportRetry,
	})
	var poll schema.PollBatchResult
	for {
		if err := workflow.ExecuteActivity(pollCtx, enrich_activities.PollBatchActivityName, schema.PollBatchInput{
			BatchID:     submitted.BatchID,
			CustomToJob: submitted.CustomToJob,
		}).Get(ctx, &poll); err != nil {
			logger.Error("poll batch failed", "batch_id", submitted.BatchID, "error", err.Error())
			return err
		}
		if poll.Ended {
			break
		}
		if err := workflow.Sleep(ctx, DefaultPollInterval); err != nil {
			return err
		}
	}

	storeCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: enrich_activities.StoreVerdictsMaxAttempts,
		},
	})
	var stored schema.StoreVerdictsResult
	if err := workflow.ExecuteActivity(storeCtx, enrich_activities.StoreVerdictsActivityName, schema.StoreVerdictsInput{
		Texts:         poll.Texts,
		PromptVersion: in.PromptVersion,
		Model:         in.Model,
	}).Get(ctx, &stored); err != nil {
		logger.Error("store verdicts failed", "error", err.Error())
		return err
	}

	if len(stored.StoredIDs) > 0 {
		if err := workflow.ExecuteActivity(storeCtx, enrich_activities.RescoreJobsActivityName, schema.RescoreJobsInput{
			JobIDs: stored.StoredIDs,
		}).Get(ctx, nil); err != nil {
			logger.Error("rescore jobs failed", "error", err.Error())
			return err
		}
	}

	logger.Info("enrich batch done",
		"batch_id", submitted.BatchID,
		"stored", len(stored.StoredIDs),
	)
	return nil
}
