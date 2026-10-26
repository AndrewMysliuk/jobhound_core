// Package enrich_activities hosts Temporal activities for EnrichBatchWorkflow.
package enrich_activities

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"

	enrichimpl "github.com/andrewmysliuk/jobhound_core/internal/enrich/impl"
	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
)

const (
	// ListMissingActivityName returns job ids with no current verdict row.
	ListMissingActivityName = "ListMissing"
	// SubmitBatchActivityName creates an Anthropic Message Batch for missing jobs.
	SubmitBatchActivityName = "SubmitBatch"
	// PollBatchActivityName checks batch status once and loads results when ended.
	PollBatchActivityName = "PollBatch"
	// StoreVerdictsActivityName coerces and inserts succeeded model replies.
	StoreVerdictsActivityName = "StoreVerdicts"
	// RescoreJobsActivityName re-evaluates matches after verdicts are stored.
	RescoreJobsActivityName = "RescoreJobs"

	// StoreVerdictsMaxAttempts is the Temporal retry budget for StoreVerdicts.
	// On the last attempt a schema-invalid body is stored as every answer unknown.
	StoreVerdictsMaxAttempts int32 = 5
)

// Rescorer is the scoring surface Enrich activities call after StoreVerdicts.
// *scoringimpl.Service implements it. Enrich does not import scoring at the module root.
type Rescorer interface {
	RescoreJobs(ctx context.Context, jobIDs []string) error
}

// Activities runs enrich batch steps. Temporal-to-domain mapping stays here, not in enrich/impl.
type Activities struct {
	Enrich  *enrichimpl.Service
	Rescore Rescorer
	Log     zerolog.Logger
}

// ListMissing returns job ids with no row for the given prompt version and model.
// Empty input Model or PromptVersion uses the enrich service defaults.
// Limit > 0 caps the result (Temporal payload budget); 0 means no cap.
func (a *Activities) ListMissing(ctx context.Context, in schema.ListMissingInput) (schema.ListMissingResult, error) {
	if a == nil || a.Enrich == nil {
		return schema.ListMissingResult{}, fmt.Errorf("enrich activity: enrich service is required")
	}
	model := in.Model
	if model == "" {
		model = a.Enrich.Model()
	}
	promptVersion := in.PromptVersion
	if promptVersion == 0 {
		promptVersion = schema.PromptVersion
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, ListMissingActivityName))
	ids, err := a.Enrich.MissingIDs(ctx, model, promptVersion)
	if err != nil {
		return schema.ListMissingResult{}, err
	}
	if in.Limit > 0 && len(ids) > in.Limit {
		ids = ids[:in.Limit]
	}
	log.Debug().Int("missing", len(ids)).Int("limit", in.Limit).Str("model", model).Msg("list missing done")
	return schema.ListMissingResult{
		JobIDs:        ids,
		PromptVersion: promptVersion,
		Model:         model,
	}, nil
}

// SubmitBatch creates one Message Batch for EnrichBatchInput.JobIDs that still lack a row.
// Pass at most schema.MaxJobsPerSubmit ids.
func (a *Activities) SubmitBatch(ctx context.Context, in schema.EnrichBatchInput) (schema.SubmitBatchResult, error) {
	if a == nil || a.Enrich == nil {
		return schema.SubmitBatchResult{}, fmt.Errorf("enrich activity: enrich service is required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, SubmitBatchActivityName))
	out, err := a.Enrich.SubmitBatch(ctx, in.JobIDs, in.Model, in.PromptVersion)
	if err != nil {
		return schema.SubmitBatchResult{}, err
	}
	log.Debug().
		Str("batch_id", out.BatchID).
		Int("submitted", len(out.SubmittedIDs)).
		Msg("submit batch done")
	return out, nil
}

// PollBatch checks processing_status once. When ended, Texts holds succeeded model replies.
func (a *Activities) PollBatch(ctx context.Context, in schema.PollBatchInput) (schema.PollBatchResult, error) {
	if a == nil || a.Enrich == nil {
		return schema.PollBatchResult{}, fmt.Errorf("enrich activity: enrich service is required")
	}
	return a.Enrich.PollBatch(ctx, in)
}

// StoreVerdicts coerces and inserts succeeded model texts.
// Schema-invalid items retry until the last attempt, then store every answer unknown.
func (a *Activities) StoreVerdicts(ctx context.Context, in schema.StoreVerdictsInput) (schema.StoreVerdictsResult, error) {
	if a == nil || a.Enrich == nil {
		return schema.StoreVerdictsResult{}, fmt.Errorf("enrich activity: enrich service is required")
	}
	if acceptUnknownOnSchemaFail(ctx) {
		in.AcceptUnknownOnSchemaFail = true
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, StoreVerdictsActivityName))
	out, err := a.Enrich.StoreVerdicts(ctx, in)
	if err != nil {
		return out, err
	}
	log.Debug().Int("stored", len(out.StoredIDs)).Msg("store verdicts done")
	return out, nil
}

// acceptUnknownOnSchemaFail is true on the final StoreVerdicts attempt.
func acceptUnknownOnSchemaFail(ctx context.Context) bool {
	if !activity.IsActivity(ctx) {
		return false
	}
	info := activity.GetInfo(ctx)
	maxAttempts := StoreVerdictsMaxAttempts
	if info.RetryPolicy != nil && info.RetryPolicy.MaximumAttempts > 0 {
		maxAttempts = info.RetryPolicy.MaximumAttempts
	}
	return info.Attempt >= maxAttempts
}

// RescoreJobs re-evaluates existing matches for the given jobs after verdicts are stored.
func (a *Activities) RescoreJobs(ctx context.Context, in schema.RescoreJobsInput) error {
	if a == nil || a.Rescore == nil {
		return fmt.Errorf("enrich activity: rescorer is required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, RescoreJobsActivityName))
	if err := a.Rescore.RescoreJobs(ctx, in.JobIDs); err != nil {
		return err
	}
	log.Debug().Int("jobs", len(in.JobIDs)).Msg("rescore done")
	return nil
}
