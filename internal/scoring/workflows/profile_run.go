package scoring_workflows

import (
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	ingest_workflows "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
	scoring_activities "github.com/andrewmysliuk/jobhound_core/internal/scoring/workflows/activities"
)

// ProfileRunWorkflowName is the registered workflow type.
const ProfileRunWorkflowName = "ProfileRunWorkflow"

// ProfileRunWorkflow ingests the profile's sources, scores the window, and marks the run succeeded.
// A locked, cooling-down, rate-limited, or missing collector child is skipped.
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
		scoredN int
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
				err := runIngestChild(laneCtx, child)
				if err == nil {
					n, scoreErr := scoreAndPublish(laneCtx, scoreMu, in, &skipped)
					if scoreErr != nil {
						if temporal.IsCanceledError(scoreErr) {
							return
						}
						failErr = scoreErr
						logger.Error("score failed", "failure_stage", "score", "error", scoreErr.Error())
						cancelFan()
						return
					}
					scoredN = n
					continue
				}
				if temporal.IsCanceledError(err) {
					return
				}
				if ChildSkipped(err) {
					logger.Warn("ingest child skipped",
						logging.FieldSourceID, child.SourceID,
						"query", child.Query,
						"workflow_id", IngestChildWorkflowID(child),
						"error", err.Error(),
					)
					if pubErr := noteSkipped(laneCtx, scoreMu, in.RunID, &skipped, &scoredN); pubErr != nil {
						if temporal.IsCanceledError(pubErr) {
							return
						}
						failErr = pubErr
						cancelFan()
						return
					}
					continue
				}
				logger.Error("ingest child failed",
					logging.FieldSourceID, child.SourceID,
					"failure_stage", "ingest",
					"workflow_id", IngestChildWorkflowID(child),
					"error", err.Error(),
				)
				failErr = err
				cancelFan()
				return
			}
		})
	}
	wg.Wait(ctx)
	if failErr != nil {
		return markFailed(ctx, in.RunID, scoredN, skipped, failErr)
	}

	var scored scoringschema.ScoreProfileResult
	if err := workflow.ExecuteActivity(ctx, scoring_activities.ScoreProfileActivityName, in).Get(ctx, &scored); err != nil {
		logger.Error("score failed", "failure_stage", "score", "error", err.Error())
		return markFailed(ctx, in.RunID, scoredN, skipped, err)
	}
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

func runIngestChild(ctx workflow.Context, child ingestschema.IngestSourceInput) error {
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:            IngestChildWorkflowID(child),
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		ParentClosePolicy:     enumspb.PARENT_CLOSE_POLICY_TERMINATE,
	})
	var out ingestschema.IngestSourceOutput
	return workflow.ExecuteChildWorkflow(ctx, ingest_workflows.IngestSourceWorkflow, child).Get(ctx, &out)
}

func scoreAndPublish(ctx workflow.Context, mu workflow.Mutex, in scoringschema.ProfileRunInput, skipped *int) (int, error) {
	if err := mu.Lock(ctx); err != nil {
		return 0, err
	}
	defer mu.Unlock()
	var scored scoringschema.ScoreProfileResult
	if err := workflow.ExecuteActivity(ctx, scoring_activities.ScoreProfileActivityName, in).Get(ctx, &scored); err != nil {
		return 0, err
	}
	if err := publishCounters(ctx, in.RunID, scored.JobsScored, *skipped); err != nil {
		return scored.JobsScored, err
	}
	return scored.JobsScored, nil
}

func noteSkipped(ctx workflow.Context, mu workflow.Mutex, runID int64, skipped, scoredN *int) error {
	if err := mu.Lock(ctx); err != nil {
		return err
	}
	defer mu.Unlock()
	*skipped++
	return publishCounters(ctx, runID, *scoredN, *skipped)
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
