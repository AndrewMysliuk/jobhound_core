// Package pipeline_activities hosts Temporal activities that call pipeline stages (no workflow imports here).
package pipeline_activities

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
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

// Activities holds dependencies for pipeline stage activities (constructed at worker wire-up).
type Activities struct {
	Clock  func() time.Time
	Scorer llm.Scorer
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
