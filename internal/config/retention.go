package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Job retention schedule (specs/006-cache-and-ingest/contracts/retention-jobs.md).
const EnvJobRetentionScheduleUpsert = "JOBHOUND_JOB_RETENTION_SCHEDULE_UPSERT"

// EnvJobRetentionDays is the retention window in whole days. Empty uses DefaultJobRetentionDays.
const EnvJobRetentionDays = "JOBHOUND_JOB_RETENTION_DAYS"

// DefaultJobRetentionDays is used when JOBHOUND_JOB_RETENTION_DAYS is unset or blank.
const DefaultJobRetentionDays = 30

// LoadJobRetentionScheduleUpsertFromEnv reads JOBHOUND_JOB_RETENTION_SCHEDULE_UPSERT.
// Default is true when unset: cmd/worker attempts to create the weekly UTC Temporal schedule.
// Set to false to skip schedule creation (e.g. multiple environments sharing one namespace).
func LoadJobRetentionScheduleUpsertFromEnv() bool {
	s := strings.TrimSpace(os.Getenv(EnvJobRetentionScheduleUpsert))
	if s == "" {
		return true
	}
	b, err := strconv.ParseBool(s)
	return err == nil && b
}

func loadJobRetentionDaysFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv(EnvJobRetentionDays))
	if raw == "" {
		return DefaultJobRetentionDays, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", EnvJobRetentionDays)
	}
	return n, nil
}
