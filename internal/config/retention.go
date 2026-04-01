package config

import (
	"os"
	"strconv"
	"strings"
)

// Job retention schedule (specs/006-cache-and-ingest/contracts/retention-jobs.md).
const EnvJobRetentionScheduleUpsert = "JOBHOUND_JOB_RETENTION_SCHEDULE_UPSERT"

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
