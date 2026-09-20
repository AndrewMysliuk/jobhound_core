package utils

import (
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/weworkremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
)

// DefaultIngestSourceIDs returns normalized source IDs aligned with cmd/worker MVPCollectors (backend-only sources).
func DefaultIngestSourceIDs() []string {
	return []string{
		ingest.NormalizeSourceID(europeremotely.SourceName),
		ingest.NormalizeSourceID(workingnomads.SourceName),
		ingest.NormalizeSourceID(builtin.SourceName),
		ingest.NormalizeSourceID(himalayas.SourceName),
		ingest.NormalizeSourceID(remotifyeurope.SourceName),
		ingest.NormalizeSourceID(weworkremotely.SourceName),
		ingest.NormalizeSourceID(wellfound.SourceName),
		ingest.NormalizeSourceID(vuejobs.SourceName),
		ingest.NormalizeSourceID(golangcafe.SourceName),
	}
}
