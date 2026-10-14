package ingest

import (
	"time"

	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// PostedAtIngestRejectDays returns the posted_at cutoff length for ingest on this source.
func PostedAtIngestRejectDays(sourceID string) int {
	if IsArchitectureSource(sourceID) {
		return scoringschema.PostedAtRejectDaysArchitecture
	}
	return scoringschema.PostedAtRejectDays
}

// SkipIngestByPostedAt is true when posted_at is known and strictly older than the ingest cutoff.
// Zero posted_at is never skipped.
func SkipIngestByPostedAt(sourceID string, posted, now time.Time) bool {
	if posted.IsZero() {
		return false
	}
	rejectDays := PostedAtIngestRejectDays(sourceID)
	if rejectDays < 1 {
		return false
	}
	cutoff := now.UTC().Add(-time.Duration(rejectDays) * 24 * time.Hour)
	return posted.UTC().Before(cutoff)
}
