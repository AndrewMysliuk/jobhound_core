// Package schema holds module-local DTOs for jobs: Temporal payloads and similar.
package schema

// JobRetentionOutput reports how many jobs were hard-deleted (RunJobRetention activity).
type JobRetentionOutput struct {
	Deleted int64
}
