package ingest

import (
	"time"

	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// SkipIngestByPostedAt is true when posted_at is known and strictly older than
// scoringschema.PostedAtRejectDays. Zero posted_at is never skipped.
func SkipIngestByPostedAt(posted, now time.Time) bool {
	if posted.IsZero() {
		return false
	}
	rejectDays := scoringschema.PostedAtRejectDays
	if rejectDays < 1 {
		return false
	}
	cutoff := now.UTC().Add(-time.Duration(rejectDays) * 24 * time.Hour)
	return posted.UTC().Before(cutoff)
}
