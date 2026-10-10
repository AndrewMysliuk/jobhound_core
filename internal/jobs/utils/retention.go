package utils

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
)

// CutoffUTC returns the hard-delete cutoff: rows with last_seen_at strictly before it are eligible.
// Days come from cfg.JobRetentionDays (JOBHOUND_JOB_RETENTION_DAYS). A non-positive value uses the default of 30.
func CutoffUTC(now time.Time, cfg config.Config) time.Time {
	days := cfg.JobRetentionDays
	if days <= 0 {
		days = config.DefaultJobRetentionDays
	}
	return now.UTC().Add(-time.Duration(days) * 24 * time.Hour)
}
