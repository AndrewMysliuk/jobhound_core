package schema

import "time"

// Match is one profile's score of one job.
type Match struct {
	ProfileID  string
	JobID      string
	Bucket     Bucket
	Score      int
	Signals    []Signal
	UserStatus UserStatus
	RunID      int64
	UpdatedAt  time.Time
}
