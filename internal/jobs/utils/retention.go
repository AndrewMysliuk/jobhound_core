package utils

import "time"

// Days is the retention window: delete jobs older than this many whole days by created_at (UTC).
// See specs/006-cache-and-ingest/contracts/retention-jobs.md.
const Days = 7

// CutoffUTC returns the cutoff for hard-delete: rows with created_at < CutoffUTC(now) are eligible.
func CutoffUTC(now time.Time) time.Time {
	return now.UTC().Add(-Days * 24 * time.Hour)
}
