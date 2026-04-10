// Package collectors defines the Collector contract and composes site-specific implementations; see internal/pipeline for stage orchestration.
package collectors

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// Collector fetches jobs from one source. Each site implements this.
type Collector interface {
	Name() string
	Fetch(ctx context.Context) ([]schema.Job, error)
}

// IncrementalCollector is optional: sources that support watermark-based fetch (006).
// Implementations return an opaque nextCursor for Postgres ingest_watermarks; empty nextCursor clears advancement for that run.
type IncrementalCollector interface {
	Collector
	FetchIncremental(ctx context.Context, cursor string) (jobs []schema.Job, nextCursor string, err error)
}

// SlotSearchFetcher scopes fetches to the user’s slot keyword (e.g. public API slot name).
// When slotQuery is empty, implementations should behave like [Collector.Fetch].
type SlotSearchFetcher interface {
	FetchWithSlotSearch(ctx context.Context, slotQuery string) ([]schema.Job, error)
}

// SessionProvider supplies browser/session state for headless collectors.
type SessionProvider interface {
	CookieFilePath() string
}
