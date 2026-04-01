// Package pipeline_activities hosts Temporal activities that call pipeline stages (no workflow imports here).
package pipeline_activities

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/llm"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
)

// PipelineStagesInput is the payload for RunPipelineStages.
type PipelineStagesInput struct {
	Jobs         []domain.Job
	BroadRules   pipeline.BroadFilterRules
	KeywordRules pipeline.KeywordRules
	Profile      string
}

// PipelineStagesOutput holds intermediate lists and the final scored jobs (stage 3).
type PipelineStagesOutput struct {
	AfterBroad    []domain.Job
	AfterKeywords []domain.Job
	Scored        []domain.ScoredJob
}

// PersistedPipelineStagesInput drives stage 1–3 with 007 per-run persistence and stage-3 cap.
type PersistedPipelineStagesInput struct {
	PipelineRunID int64
	Jobs          []domain.Job
	BroadRules    pipeline.BroadFilterRules
	KeywordRules  pipeline.KeywordRules
	Profile       string
	// BroadFilterKeyHash is optional SHA-256 hex of the canonical broad filter key (006); persisted on pipeline_runs when non-empty.
	BroadFilterKeyHash string
	// Stage3SentJobIDs is optional idempotency for Temporal retries: job IDs already sent to the scorer in this workflow execution.
	Stage3SentJobIDs []string
}

// PersistedPipelineStagesOutput mirrors [PipelineStagesOutput] and returns Stage3SentJobIDs for workflow retry bookkeeping.
type PersistedPipelineStagesOutput struct {
	AfterBroad       []domain.Job
	AfterKeywords    []domain.Job
	Scored           []domain.ScoredJob
	Stage3SentJobIDs []string
}

// Activities holds dependencies for pipeline stage activities (constructed at worker wire-up).
type Activities struct {
	Clock  func() time.Time
	Scorer llm.Scorer
	Runs   pipeline.PipelineRunRepository
	Jobs   jobs.JobRepository
	// Stage3MaxJobsPerRun caps stage-3 batch per persisted run; zero uses [pipeutils.MaxStage3JobsPerPipelineRunExecution].
	Stage3MaxJobsPerRun int
}

// RunPipelineStages applies stage 1 → 2 → 3 in order. Requires a non-nil Scorer.
func (a *Activities) RunPipelineStages(ctx context.Context, in PipelineStagesInput) (*PipelineStagesOutput, error) {
	if a == nil || a.Scorer == nil {
		return nil, fmt.Errorf("pipeline activities: nil Activities or Scorer")
	}
	clock := a.Clock
	if clock == nil {
		clock = time.Now
	}
	stage1, err := pipeutils.ApplyBroadFilter(clock, in.BroadRules, in.Jobs)
	if err != nil {
		return nil, err
	}
	stage2 := pipeutils.ApplyKeywordFilter(stage1, in.KeywordRules)
	scored, err := pipeutils.ScoreJobs(ctx, in.Profile, stage2, a.Scorer)
	if err != nil {
		return nil, err
	}
	return &PipelineStagesOutput{
		AfterBroad:    stage1,
		AfterKeywords: stage2,
		Scored:        scored,
	}, nil
}

// RunPersistedPipelineStages applies stage 1–2 in memory, persists REJECTED_STAGE_2 / PASSED_STAGE_2 per job
// that passed stage 1 (004 omission model: stage-1 drops get no pipeline_run_jobs row), loads PASSED_STAGE_2
// candidates for the run, selects at most N for stage 3 with optional Stage3SentJobIDs exclusion, scores via
// [llm.Scorer], and persists PASSED_STAGE_3 / REJECTED_STAGE_3 using [pipeutils.TerminalRunJobStatusFromScoredJob].
// LLM errors from Score abort the activity with error (004 / plan D5).
func (a *Activities) RunPersistedPipelineStages(ctx context.Context, in PersistedPipelineStagesInput) (*PersistedPipelineStagesOutput, error) {
	if a == nil || a.Scorer == nil {
		return nil, fmt.Errorf("pipeline activities: nil Activities or Scorer")
	}
	if a.Runs == nil || a.Jobs == nil {
		return nil, fmt.Errorf("pipeline activities: RunPersistedPipelineStages requires Runs and Jobs repositories")
	}
	if in.PipelineRunID <= 0 {
		return nil, fmt.Errorf("pipeline activities: pipeline run id is required")
	}
	if in.BroadFilterKeyHash != "" {
		if err := a.Runs.SetBroadFilterKeyHash(ctx, in.PipelineRunID, in.BroadFilterKeyHash); err != nil {
			return nil, err
		}
	}
	clock := a.Clock
	if clock == nil {
		clock = time.Now
	}

	stage1, err := pipeutils.ApplyBroadFilter(clock, in.BroadRules, in.Jobs)
	if err != nil {
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
			return nil, err
		}
	}

	candidates, err := a.Runs.ListPassedStage2JobIDs(ctx, in.PipelineRunID)
	if err != nil {
		return nil, err
	}
	exclude := make(map[string]struct{})
	for _, id := range in.Stage3SentJobIDs {
		if id != "" {
			exclude[id] = struct{}{}
		}
	}
	selected := pipeutils.SelectStage3JobIDs(candidates, exclude, a.Stage3MaxJobsPerRun)

	sentIDs := append([]string(nil), in.Stage3SentJobIDs...)
	var scored []domain.ScoredJob
	for _, id := range selected {
		job, err := a.Jobs.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load job %q: %w", id, err)
		}
		sj, err := a.Scorer.Score(ctx, in.Profile, job)
		if err != nil {
			return nil, fmt.Errorf("score job %q: %w", id, err)
		}
		terminal := pipeutils.TerminalRunJobStatusFromScoredJob(sj)
		if err := a.Runs.SetRunJobStatus(ctx, in.PipelineRunID, id, terminal); err != nil {
			return nil, err
		}
		scored = append(scored, sj)
		sentIDs = append(sentIDs, id)
	}

	return &PersistedPipelineStagesOutput{
		AfterBroad:       stage1,
		AfterKeywords:    stage2,
		Scored:           scored,
		Stage3SentJobIDs: sentIDs,
	}, nil
}
