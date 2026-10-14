package utils

import "time"

// PostedBeforeCutoff reports whether posted is set and strictly before now minus rejectDays full days.
func PostedBeforeCutoff(posted, now time.Time, rejectDays int) bool {
	if posted.IsZero() || now.IsZero() || rejectDays < 1 {
		return false
	}
	cutoff := now.UTC().Add(-time.Duration(rejectDays) * 24 * time.Hour)
	return posted.UTC().Before(cutoff)
}
