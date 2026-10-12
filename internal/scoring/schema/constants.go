package schema

const (
	QueryPoints   = 1
	PenaltyPoints = -10
	// ScoreWindowDays is the first_seen_at window measured against profile_runs.started_at.
	ScoreWindowDays = 10
	// PostedAtRejectDays rejects a known posted_at strictly older than this many days.
	// A zero posted_at is not rejected. The clock is the run's started_at.
	PostedAtRejectDays = 30
)
