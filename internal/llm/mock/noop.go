package mock

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

// Scorer returns a fixed zero score per job without I/O (tests and bootstrap wiring).
type Scorer struct{}

func (Scorer) Score(_ context.Context, _ string, job domain.Job) (domain.ScoredJob, error) {
	return domain.ScoredJob{Job: job, Score: 0, Reason: ""}, nil
}
