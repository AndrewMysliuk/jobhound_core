// Package schema holds module-local DTOs for ingest: Temporal workflow/activity payloads.
package schema

// IngestSourceInput selects a normalized source id and optional cooldown bypass (Temporal RunIngestSource).
type IngestSourceInput struct {
	SourceID        string
	ExplicitRefresh bool
	// Query is the search string or Wellfound role slug for this ingest child.
	// Empty means catalog fetch.
	Query string
}

// IngestSourceOutput summarizes ingest work for observability.
type IngestSourceOutput struct {
	JobsWritten       int
	JobsSkipped       int
	UsedIncremental   bool
	WatermarkAdvanced bool
}
