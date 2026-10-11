// Package schema holds module-local DTOs for ingest: Temporal workflow/activity payloads.
package schema

// IngestSourceInput selects a normalized source id and optional cooldown bypass (Temporal RunIngestSource).
type IngestSourceInput struct {
	SourceID        string
	ExplicitRefresh bool
	// Query is one search string. Empty is the catalog lock segment.
	Query string
	// Queries is the full profile query list for a union search.
	// Set only for Consider. Query stays empty, so the lock segment is catalog.
	Queries []string
}

// IngestSourceOutput summarizes ingest work for observability.
type IngestSourceOutput struct {
	JobsWritten       int
	JobsSkipped       int
	UsedIncremental   bool
	WatermarkAdvanced bool
}
