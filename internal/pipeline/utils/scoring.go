package utils

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/llm"
)

// ScoreJobs runs stage 3 on each job in order. It stops and returns the first error from the scorer.
func ScoreJobs(ctx context.Context, profile string, jobs []domain.Job, scorer llm.Scorer) ([]domain.ScoredJob, error) {
	out := make([]domain.ScoredJob, 0, len(jobs))
	for _, j := range jobs {
		sj, err := scorer.Score(ctx, profile, j)
		if err != nil {
			return nil, err
		}
		out = append(out, sj)
	}
	return out, nil
}
