package slots

import (
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
)

// DefaultIngestSourceIDs returns normalized source ids aligned with cmd/worker MVPCollectors (009: backend-only sources).
func DefaultIngestSourceIDs() []string {
	return []string{
		ingest.NormalizeSourceID(europeremotely.SourceName),
		ingest.NormalizeSourceID(workingnomads.SourceName),
	}
}
