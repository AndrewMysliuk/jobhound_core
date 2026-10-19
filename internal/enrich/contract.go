// Package enrich is the vacancy-facts module: contracts at the root, data model under schema/.
package enrich

import (
	"context"
	"errors"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
)

// API is the enrich use-case surface. Implementations do not import Temporal.
type API interface {
	SubmitMissing(ctx context.Context, jobIDs []string) error
	MissingIDs(ctx context.Context, model string, promptVersion int) ([]string, error)
	Verdict(ctx context.Context, jobID, model string, promptVersion int, inputHash string) (schema.Verdict, error)
	Statuses(ctx context.Context, jobIDs []string) (map[string]schema.FactsStatus, error)
}

var (
	ErrNotFound = errors.New("verdict not found")
)
