package utils

import (
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
)

// DefaultIngestSourceIDs returns normalized source IDs aligned with cmd/worker MVPCollectors (009: backend-only sources).
func DefaultIngestSourceIDs() []string {
	return []string{
		ingest.NormalizeSourceID(europeremotely.SourceName),
		ingest.NormalizeSourceID(workingnomads.SourceName),
	}
}
