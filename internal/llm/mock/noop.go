package mock

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// Scorer returns a fixed zero score per job without I/O (tests and bootstrap wiring).
type Scorer struct{}

func (Scorer) Score(_ context.Context, _ string, job schema.Job) (schema.ScoredJob, error) {
	return schema.ScoredJob{Job: job, Score: 0, Reason: ""}, nil
}
