// Package pipeline_activities hosts Temporal activities that call pipeline stages (no workflow imports here).
package pipeline_activities

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/llm"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/rs/zerolog"
)

// Matches [pipeline_workflows.RunPipelineStagesActivityName] (activities must not import the parent workflows package).
const runPipelineStagesActivityName = "RunPipelineStagesActivity"

// Activities holds dependencies for pipeline stage activities (constructed at worker wire-up).
type Activities struct {
	Clock  func() time.Time
	Scorer llm.Scorer
	Runs   pipeline.PipelineRunRepository
	Jobs   jobs.JobRepository
	// Stage3MaxJobsPerRun caps stage-3 batch per persisted run; zero uses [pipeutils.MaxStage3JobsPerPipelineRunExecution].
	Stage3MaxJobsPerRun int
	Log                 zerolog.Logger
}

// RunPipelineStages applies stage 1 → 2 → 3 in order. Requires a non-nil Scorer.
func (a *Activities) RunPipelineStages(ctx context.Context, in pipelineschema.PipelineStagesInput) (*pipelineschema.PipelineStagesOutput, error) {
	if a == nil || a.Scorer == nil {
		return nil, fmt.Errorf("pipeline activities: nil Activities or Scorer")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, runPipelineStagesActivityName))
	log.Debug().Int("job_count", len(in.Jobs)).Msg("run pipeline stages start")
	clock := a.Clock
	if clock == nil {
		clock = time.Now
	}
	stage1, err := pipeutils.ApplyBroadFilter(clock, in.BroadRules, in.Jobs)
	if err != nil {
		log.Error().Err(err).Msg("broad filter")
		return nil, err
	}
	stage2 := pipeutils.ApplyKeywordFilter(stage1, in.KeywordRules)
	scored, err := pipeutils.ScoreJobs(ctx, in.Profile, stage2, a.Scorer)
	if err != nil {
		log.Error().Err(err).Msg("score jobs")
		return nil, err
	}
	log.Debug().Int("after_broad", len(stage1)).Int("after_keywords", len(stage2)).Int("scored", len(scored)).Msg("run pipeline stages done")
	return &pipelineschema.PipelineStagesOutput{
		AfterBroad:    stage1,
		AfterKeywords: stage2,
		Scored:        scored,
	}, nil
}

// RunPersistPipelineStage2 applies stage 1–2 in memory and persists REJECTED_STAGE_2 / PASSED_STAGE_2 per job
// that passed stage 1 (004 omission model).
func (a *Activities) RunPersistPipelineStage2(ctx context.Context, in pipelineschema.PersistPipelineStage2Input) (*pipelineschema.PersistPipelineStage2Output, error) {
	ctx = logging.WithPipelineRunIDInt64(ctx, in.PipelineRunID)
	if a == nil || a.Runs == nil {
		return nil, fmt.Errorf("pipeline activities: RunPersistPipelineStage2 requires Runs repository")
	}
	if in.PipelineRunID <= 0 {
		return nil, fmt.Errorf("pipeline activities: pipeline run id is required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, manualschema.PersistPipelineStage2ActivityName))
	log.Debug().Int("job_count", len(in.Jobs)).Msg("persist stage 2 start")
	if in.BroadFilterKeyHash != "" {
		if err := a.Runs.SetBroadFilterKeyHash(ctx, in.PipelineRunID, in.BroadFilterKeyHash); err != nil {
			log.Error().Err(err).Msg("set broad filter key hash")
			return nil, err
		}
	}
	clock := a.Clock
	if clock == nil {
		clock = time.Now
	}

	stage1, err := pipeutils.ApplyBroadFilter(clock, in.BroadRules, in.Jobs)
	if err != nil {
		log.Error().Err(err).Msg("broad filter")
		return nil, err
	}
	stage2 := pipeutils.ApplyKeywordFilter(stage1, in.KeywordRules)
	stage2OK := make(map[string]struct{}, len(stage2))
	for _, j := range stage2 {
		stage2OK[j.ID] = struct{}{}
	}

	for _, j := range stage1 {
		st := pipeline.RunJobRejectedStage2
		if _, ok := stage2OK[j.ID]; ok {
			st = pipeline.RunJobPassedStage2
		}
		if err := a.Runs.SetRunJobStatus(ctx, in.PipelineRunID, j.ID, st); err != nil {
			log.Error().Err(err).Str("job_id", j.ID).Msg("set run job status")
			return nil, err
		}
	}

	log.Debug().Int("after_broad", len(stage1)).Int("after_keywords", len(stage2)).Msg("persist stage 2 done")
	return &pipelineschema.PersistPipelineStage2Output{
		AfterBroad:    stage1,
		AfterKeywords: stage2,
	}, nil
}

// RunPersistPipelineStage3 loads PASSED_STAGE_2 candidates for the run (ordered by posted_at DESC per repository),
// selects at most N for stage 3 with optional Stage3SentJobIDs exclusion, scores via [llm.Scorer], and persists
// PASSED_STAGE_3 / REJECTED_STAGE_3. LLM errors abort the activity (004 / 007).
func (a *Activities) RunPersistPipelineStage3(ctx context.Context, in pipelineschema.PersistPipelineStage3Input) (*pipelineschema.PersistPipelineStage3Output, error) {
	ctx = logging.WithPipelineRunIDInt64(ctx, in.PipelineRunID)
	if a == nil || a.Scorer == nil {
		return nil, fmt.Errorf("pipeline activities: nil Activities or Scorer")
	}
	if a.Runs == nil || a.Jobs == nil {
		return nil, fmt.Errorf("pipeline activities: RunPersistPipelineStage3 requires Runs and Jobs repositories")
	}
	if in.PipelineRunID <= 0 {
		return nil, fmt.Errorf("pipeline activities: pipeline run id is required")
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, manualschema.PersistPipelineStage3ActivityName))

	candidates, err := a.Runs.ListPassedStage2JobIDs(ctx, in.PipelineRunID)
	if err != nil {
		log.Error().Err(err).Msg("list passed stage 2 job ids")
		return nil, err
	}
	exclude := make(map[string]struct{})
	for _, id := range in.Stage3SentJobIDs {
		if id != "" {
			exclude[id] = struct{}{}
		}
	}
	policy := a.Stage3MaxJobsPerRun
	if policy <= 0 {
		policy = pipeutils.MaxStage3JobsPerPipelineRunExecution
	}
	capN := policy
	if in.MaxJobs > 0 && in.MaxJobs < capN {
		capN = in.MaxJobs
	}
	selected := pipeutils.SelectStage3JobIDs(candidates, exclude, capN)
	log.Debug().Int("candidates", len(candidates)).Int("selected", len(selected)).Int("max_jobs", in.MaxJobs).Msg("persist stage 3 start")

	// SetRunJobStatus is idempotent for terminal rows; GetRunJobStatus skips duplicate LLM work on retry.
	sentIDs := append([]string(nil), in.Stage3SentJobIDs...)
	var scored []schema.ScoredJob
	for _, id := range selected {
		if cur, ok, err := a.Runs.GetRunJobStatus(ctx, in.PipelineRunID, id); err != nil {
			log.Error().Err(err).Str("job_id", id).Msg("get run job status")
			return nil, err
		} else if ok && (cur == pipeline.RunJobPassedStage3 || cur == pipeline.RunJobRejectedStage3) {
			continue
		}
		job, err := a.Jobs.GetByID(ctx, id)
		if err != nil {
			err = fmt.Errorf("load job %q: %w", id, err)
			log.Error().Err(err).Str("job_id", id).Msg("load job")
			return nil, err
		}
		sj, err := a.Scorer.Score(ctx, in.Profile, job)
		if err != nil {
			err = fmt.Errorf("score job %q: %w", id, err)
			log.Error().Err(err).Str("job_id", id).Msg("score job")
			return nil, err
		}
		terminal := pipeutils.TerminalRunJobStatusFromScoredJob(sj)
		if err := a.Runs.SetRunJobStatus(ctx, in.PipelineRunID, id, terminal); err != nil {
			log.Error().Err(err).Str("job_id", id).Msg("set run job status")
			return nil, err
		}
		if err := a.Runs.SetRunJobStage3Rationale(ctx, in.PipelineRunID, id, sj.Reason); err != nil {
			log.Error().Err(err).Str("job_id", id).Msg("set stage 3 rationale")
			return nil, err
		}
		scored = append(scored, sj)
		sentIDs = append(sentIDs, id)
	}

	log.Debug().Int("scored", len(scored)).Msg("persist stage 3 done")
	return &pipelineschema.PersistPipelineStage3Output{
		Scored:           scored,
		Stage3SentJobIDs: sentIDs,
	}, nil
}
