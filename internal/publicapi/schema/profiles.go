package schema

import "time"

// ProfileListItem is one profile in GET /api/v1/profiles.
// Exclude lists and penalties are not on this type.
type ProfileListItem struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Domain  string   `json:"domain"`
	Sources []string `json:"sources"`
	Queries []string `json:"queries"`
}

// ProfileListResponse is the GET /api/v1/profiles 200 body.
type ProfileListResponse struct {
	Profiles []ProfileListItem `json:"profiles"`
}

// ProfileRunResponse is the run body for POST /api/v1/profiles/{profile_id}/runs
// and GET /api/v1/profiles/{profile_id}/runs/latest. There is no run request body.
type ProfileRunResponse struct {
	RunID          int64      `json:"run_id"`
	ProfileID      string     `json:"profile_id"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	JobsScored     int        `json:"jobs_scored"`
	SourcesSkipped int        `json:"sources_skipped"`
}

// ProfileJobLocation is the display location on a scored job.
type ProfileJobLocation struct {
	Type      string   `json:"type"`
	Regions   []string `json:"regions"`
	Countries []string `json:"countries"`
	Timezone  string   `json:"timezone"`
	Raw       string   `json:"raw"`
}

// ProfileJobSignal is one fired phrase on a scored job.
type ProfileJobSignal struct {
	Code   string   `json:"code"`
	Points int      `json:"points"`
	Terms  []string `json:"terms,omitempty"`
}

// ProfileJob is one job entry in the profile job page and the PATCH 200 body.
type ProfileJob struct {
	JobID       string             `json:"job_id"`
	Title       string             `json:"title"`
	Company     string             `json:"company"`
	URL         string             `json:"url"`
	ApplyURL    string             `json:"apply_url"`
	Location    ProfileJobLocation `json:"location"`
	Sources     []string           `json:"sources"`
	PostedAt    *time.Time         `json:"posted_at"`
	FirstSeenAt time.Time          `json:"first_seen_at"`
	LastSeenAt  time.Time          `json:"last_seen_at"`
	Bucket      string             `json:"bucket"`
	Score       int                `json:"score"`
	Signals     []ProfileJobSignal `json:"signals"`
	UserStatus  string             `json:"user_status"`
}

// ProfileJobPage is the GET /api/v1/profiles/{profile_id}/jobs 200 body.
type ProfileJobPage struct {
	Page  int          `json:"page"`
	Limit int          `json:"limit"`
	Total int          `json:"total"`
	Jobs  []ProfileJob `json:"jobs"`
}

// PatchUserStatusRequest is the PATCH /api/v1/profiles/{profile_id}/jobs/{job_id} body.
// Handlers decode it with DisallowUnknownFields.
type PatchUserStatusRequest struct {
	UserStatus string `json:"user_status"`
}
