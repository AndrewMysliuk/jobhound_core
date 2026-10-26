package scoring_workflows

import (
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	companiesschema "github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	companies_workflows "github.com/andrewmysliuk/jobhound_core/internal/companies/workflows"
	companies_activities "github.com/andrewmysliuk/jobhound_core/internal/companies/workflows/activities"
	enrichschema "github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	enrich_workflows "github.com/andrewmysliuk/jobhound_core/internal/enrich/workflows"
	enrich_activities "github.com/andrewmysliuk/jobhound_core/internal/enrich/workflows/activities"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	ingest_workflows "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	scoring_activities "github.com/andrewmysliuk/jobhound_core/internal/scoring/workflows/activities"
)

// ProfileRunWorkflowName is the registered workflow type.
const ProfileRunWorkflowName = "ProfileRunWorkflow"

// ProfileRunWorkflow ingests the profile's sources, discovers/resolves/polls companies,
// scores the window, kicks off enrich, and marks the run succeeded.
// A failed ingest child (any board error, lock, cooldown, rate limit, missing collector)
// is counted in sources_skipped and does not fail the run.
// Resolve children use ParentClosePolicy abandon; a failed resolve does not fail the run.
// Enrich is fire-and-forget: the parent does not wait for the batch.
func ProfileRunWorkflow(ctx workflow.Context, in scoringschema.ProfileRunInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("profile run start",
		logging.FieldWorkflow, ProfileRunWorkflowName,
		"profile_id", in.ProfileID,
		"profile_run_id", in.RunID,
	)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 20 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	var profile profileschema.Profile
	if err := workflow.ExecuteActivity(ctx, scoring_activities.LoadProfileActivityName, in.ProfileID).Get(ctx, &profile); err != nil {
		logger.Error("load profile failed", "failure_stage", "ingest", "error", err.Error())
		return markFailed(ctx, in.RunID, 0, 0, err)
	}

	fanCtx, cancelFan := workflow.WithCancel(ctx)
	defer cancelFan()

	var (
		skipped int
		failErr error
	)
	scoreMu := workflow.NewMutex(ctx)
	wg := workflow.NewWaitGroup(ctx)
	for _, lane := range IngestLanes(profile) {
		lane := lane
		wg.Add(1)
		workflow.GoNamed(fanCtx, "ingest-lane-"+lane.Name, func(laneCtx workflow.Context) {
			defer wg.Done()
			for _, child := range lane.Children {
				if laneCtx.Err() != nil || failErr != nil {
					return
				}
				out, err := runIngestChild(laneCtx, child)
				if err == nil {
					// No mid-lane ScoreProfile: one final pass runs after companies stages.
					if !out.Skipped {
						continue
					}
					if pubErr := noteSkipped(laneCtx, scoreMu, in.RunID, &skipped); pubErr != nil {
						if temporal.IsCanceledError(pubErr) {
							return
						}
						failErr = pubErr
						cancelFan()
						return
					}
					continue
				}
				if temporal.IsCanceledError(err) {
					return
				}
				logger.Warn("ingest child skipped",
					logging.FieldSourceID, child.SourceID,
					"query", child.Query,
					"workflow_id", IngestChildWorkflowID(child),
					"error", err.Error(),
				)
				if pubErr := noteSkipped(laneCtx, scoreMu, in.RunID, &skipped); pubErr != nil {
					if temporal.IsCanceledError(pubErr) {
						return
					}
					failErr = pubErr
					cancelFan()
					return
				}
			}
		})
	}
	wg.Wait(ctx)
	if failErr != nil {
		return markFailed(ctx, in.RunID, 0, skipped, failErr)
	}

	if profile.ATSPolling {
		if err := runCompaniesStages(ctx, in); err != nil {
			logger.Error("companies stages failed", "failure_stage", "discover", "error", err.Error())
			return markFailed(ctx, in.RunID, 0, skipped, err)
		}
	} else {
		logger.Info("companies stages skipped", "profile_id", profile.ID)
	}

	var scored scoringschema.ScoreProfileResult
	if err := workflow.ExecuteActivity(ctx, scoring_activities.ScoreProfileActivityName, in).Get(ctx, &scored); err != nil {
		logger.Error("score failed", "failure_stage", "score", "error", err.Error())
		return markFailed(ctx, in.RunID, 0, skipped, err)
	}
	startEnrichBatch(ctx)
	err := workflow.ExecuteActivity(ctx, scoring_activities.MarkRunFinishedActivityName, scoringschema.MarkRunFinishedInput{
		RunID:          in.RunID,
		Status:         scoringschema.RunStatusSucceeded,
		JobsScored:     scored.JobsScored,
		SourcesSkipped: skipped,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

// runCompaniesStages: discover → resolve children → poll (before score).
// A failed resolve child does not fail the run. Poll waits so score sees ATS text.
func runCompaniesStages(ctx workflow.Context, in scoringschema.ProfileRunInput) error {
	logger := workflow.GetLogger(ctx)

	var discovered companiesschema.UpsertCompaniesFromJobsResult
	if err := workflow.ExecuteActivity(ctx, companies_activities.UpsertCompaniesFromJobsActivityName, companiesschema.UpsertCompaniesFromJobsInput{
		RunID: in.RunID,
	}).Get(ctx, &discovered); err != nil {
		logger.Error("discover failed",
			"failure_stage", string(companiesschema.FailureStageDiscover),
			"error", err.Error(),
		)
		return err
	}
	if err := addATSCounters(ctx, in.RunID, companies_workflows.ATSCountersFromDiscover(discovered)); err != nil {
		return err
	}

	var pending companiesschema.ListPendingResolveResult
	if err := workflow.ExecuteActivity(ctx, companies_activities.ListPendingResolveActivityName, companiesschema.ListPendingResolveInput{}).Get(ctx, &pending); err != nil {
		logger.Error("list pending resolve failed",
			"failure_stage", string(companiesschema.FailureStageResolve),
			"error", err.Error(),
		)
		// Do not fail the run: resolve is best-effort like enrich.
		pending = companiesschema.ListPendingResolveResult{}
	}

	resolveWg := workflow.NewWaitGroup(ctx)
	for _, key := range pending.CompanyKeys {
		key := key
		resolveWg.Add(1)
		workflow.GoNamed(ctx, "resolve-"+key, func(gctx workflow.Context) {
			defer resolveWg.Done()
			out, err := startResolveChild(gctx, key)
			if err != nil {
				logger.Warn("resolve child failed",
					"failure_stage", string(companiesschema.FailureStageResolve),
					"company_key", key,
					"workflow_id", companies_workflows.ResolveWorkflowID(key),
					"error", err.Error(),
				)
				return
			}
			if err := addATSCounters(gctx, in.RunID, companies_workflows.ATSCountersFromResolve(out)); err != nil {
				logger.Warn("resolve counters skipped",
					"failure_stage", string(companiesschema.FailureStageResolve),
					"company_key", key,
					"error", err.Error(),
				)
			}
		})
	}
	resolveWg.Wait(ctx)

	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:            companies_workflows.PollWorkflowIDFromRun(in.RunID),
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		ParentClosePolicy:     enumspb.PARENT_CLOSE_POLICY_ABANDON,
	})
	var pollOut companiesschema.PollCompaniesResult
	err := workflow.ExecuteChildWorkflow(childCtx, companies_workflows.PollCompaniesWorkflow, companiesschema.PollCompaniesPayload{
		RunID: companies_workflows.RunIDString(in.RunID),
	}).Get(ctx, &pollOut)
	if err != nil {
		logger.Warn("poll companies skipped",
			"failure_stage", string(companiesschema.FailureStagePoll),
			"error", err.Error(),
		)
		return nil
	}
	return addATSCounters(ctx, in.RunID, companies_workflows.ATSCountersFromPollWorkflow(pollOut))
}

func startResolveChild(ctx workflow.Context, companyKey string) (companiesschema.ResolveCompanyResult, error) {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:            companies_workflows.ResolveWorkflowID(companyKey),
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		ParentClosePolicy:     enumspb.PARENT_CLOSE_POLICY_ABANDON,
	})
	var out companiesschema.ResolveCompanyResult
	err := workflow.ExecuteChildWorkflow(childCtx, companies_workflows.ResolveCompanyWorkflow, companiesschema.ResolveCompanyPayload{
		CompanyKey: companyKey,
	}).Get(ctx, &out)
	return out, err
}

func addATSCounters(ctx workflow.Context, runID int64, delta scoringschema.RunATSCounters) error {
	if delta == (scoringschema.RunATSCounters{}) {
		return nil
	}
	return workflow.ExecuteActivity(ctx, scoring_activities.AddATSCountersActivityName, scoringschema.AddATSCountersInput{
		RunID: runID,
		Delta: delta,
	}).Get(ctx, nil)
}

// startEnrichBatch probes for at least one missing verdict and fire-and-forgets
// EnrichBatchWorkflow in drain mode (empty JobIDs; child pages ListMissing).
// Empty probe skips the child. A failed ListMissing or child start does not fail the profile run.
func startEnrichBatch(ctx workflow.Context) {
	logger := workflow.GetLogger(ctx)
	var probe enrichschema.ListMissingResult
	if err := workflow.ExecuteActivity(ctx, enrich_activities.ListMissingActivityName, enrichschema.ListMissingInput{
		Limit: 1,
	}).Get(ctx, &probe); err != nil {
		logger.Warn("enrich list missing skipped", "error", err.Error())
		return
	}
	if len(probe.JobIDs) == 0 {
		return
	}
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:            enrich_workflows.DrainWorkflowID(probe.PromptVersion, probe.Model),
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		ParentClosePolicy:     enumspb.PARENT_CLOSE_POLICY_ABANDON,
	})
	fut := workflow.ExecuteChildWorkflow(childCtx, enrich_workflows.EnrichBatchWorkflow, enrichschema.EnrichBatchInput{
		PromptVersion: probe.PromptVersion,
		Model:         probe.Model,
	})
	// Wait only for the child to be scheduled; do not wait for it to finish.
	if err := fut.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
		logger.Warn("enrich child start skipped", "error", err.Error())
	}
}

func runIngestChild(ctx workflow.Context, child ingestschema.IngestSourceInput) (ingestschema.IngestSourceOutput, error) {
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:            IngestChildWorkflowID(child),
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		ParentClosePolicy:     enumspb.PARENT_CLOSE_POLICY_TERMINATE,
	})
	var out ingestschema.IngestSourceOutput
	err := workflow.ExecuteChildWorkflow(ctx, ingest_workflows.IngestSourceWorkflow, child).Get(ctx, &out)
	return out, err
}

func noteSkipped(ctx workflow.Context, mu workflow.Mutex, runID int64, skipped *int) error {
	if err := mu.Lock(ctx); err != nil {
		return err
	}
	defer mu.Unlock()
	*skipped++
	// jobs_scored stays 0 until the final ScoreProfile after companies stages.
	return publishCounters(ctx, runID, 0, *skipped)
}

func publishCounters(ctx workflow.Context, runID int64, jobsScored, sourcesSkipped int) error {
	return workflow.ExecuteActivity(ctx, scoring_activities.SetRunCountersActivityName, scoringschema.RunCountersInput{
		RunID:          runID,
		JobsScored:     jobsScored,
		SourcesSkipped: sourcesSkipped,
	}).Get(ctx, nil)
}

func markFailed(ctx workflow.Context, runID int64, jobsScored, sourcesSkipped int, cause error) error {
	err := workflow.ExecuteActivity(ctx, scoring_activities.MarkRunFinishedActivityName, scoringschema.MarkRunFinishedInput{
		RunID:          runID,
		Status:         scoringschema.RunStatusFailed,
		JobsScored:     jobsScored,
		SourcesSkipped: sourcesSkipped,
	}).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("mark run failed", "error", err.Error())
	}
	return cause
}
