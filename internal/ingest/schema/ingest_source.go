// Package schema holds module-local DTOs for ingest: Temporal workflow/activity payloads.
package schema

import "github.com/google/uuid"

// IngestSourceInput selects a normalized source id and optional cooldown bypass (Temporal RunIngestSource).
type IngestSourceInput struct {
	// SlotID scopes watermarks and ingest bookkeeping per search slot (006 / product draft §2–3).
	SlotID          uuid.UUID
	SourceID        string
	ExplicitRefresh bool
	// SlotSearchQuery is the slot display name used as the per-source search keyword (public API POST /slots).
	// Empty means full default listing for sources that support unscoped fetch.
	SlotSearchQuery string
}

// IngestSourceOutput summarizes ingest work for observability.
type IngestSourceOutput struct {
	JobsWritten       int
	JobsSkipped       int
	JobsFilteredOut   int
	UsedIncremental   bool
	WatermarkAdvanced bool
}
