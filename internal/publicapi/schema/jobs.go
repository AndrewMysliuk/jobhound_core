package schema

import "time"

// DefaultJobListLimit is the default page size for GET …/stages/*/jobs (plan.md; max 100).
const DefaultJobListLimit = 50

// MaxJobListLimit is the maximum allowed limit query value.
const MaxJobListLimit = 100

// JobListItem is one row in paginated job lists (contracts/http-public-api.md §4.6).
// stage_3_rationale is always JSON null when absent (plan.md D4: null, not omit).
// Status is pipeline_run_jobs.status for GET …/stages/2|3/jobs only; omitted for stage 1.
type JobListItem struct {
	JobID           string     `json:"job_id"`
	Title           string     `json:"title"`
	Company         string     `json:"company"`
	SourceID        string     `json:"source_id"`
	ApplyURL        string     `json:"apply_url"`
	FirstSeenAt     time.Time  `json:"first_seen_at"`
	PostedAt        *time.Time `json:"posted_at"`
	Status          *string    `json:"status,omitempty"`
	Stage3Rationale *string    `json:"stage_3_rationale"`
}

// JobListResponse is GET …/stages/{1|2|3}/jobs 200 body.
type JobListResponse struct {
	Items []JobListItem `json:"items"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
	Total int           `json:"total"`
}
