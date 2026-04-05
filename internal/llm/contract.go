// Package llm defines contracts for LLM-backed steps (scoring, future extractors, etc.)
// and provider implementations under llm/<vendor>.
package llm

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// Scorer scores one item (e.g. a job) against user profile text. Implementations may call
// Anthropic, OpenAI, local models, etc.; wire-up supplies keys and models, not os.Getenv here.
type Scorer interface {
	Score(ctx context.Context, profile string, job schema.Job) (schema.ScoredJob, error)
}
