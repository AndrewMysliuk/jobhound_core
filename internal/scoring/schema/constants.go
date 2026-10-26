package schema

const (
	// ScoreWindowDays is the first_seen_at window measured against profile_runs.started_at.
	ScoreWindowDays = 10
	// PostedAtRejectDays rejects a known posted_at strictly older than this many days.
	// One threshold for every profile and every source. A zero posted_at is not rejected.
	// Scoring clocks against the run's started_at; ingest and board upsert use write time.
	PostedAtRejectDays = 30
)
