// Package collectors defines the Collector contract and composes site-specific implementations.
package collectors

import (
	"context"
	"errors"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// Collector fetches jobs from one source. Each site implements this.
type Collector interface {
	Name() string
	Fetch(ctx context.Context) ([]schema.Job, error)
}

// QueryFetcher fetches listings for one search string (keyword or Wellfound role slug).
// When query is empty, implementations should behave like [Collector.Fetch].
type QueryFetcher interface {
	FetchWithQuery(ctx context.Context, query string) ([]schema.Job, error)
}

// UnionQueryFetcher searches once with every profile query.
// Consider maps those words onto skill ids and sends them as one union.
type UnionQueryFetcher interface {
	FetchUnion(ctx context.Context, queries []string) ([]schema.Job, error)
}

// ErrTenantUnavailable means this fund's host or collection is gone.
// The profile run skips that source. Other tenants still run.
var ErrTenantUnavailable = errors.New("collector tenant unavailable")
