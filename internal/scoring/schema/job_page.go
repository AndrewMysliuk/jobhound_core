package schema

import (
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// ListJobsParams filters API.ListJobs.
// An empty Bucket means PASSED. An empty UserStatus means NEW.
type ListJobsParams struct {
	ProfileID  string
	Bucket     Bucket
	UserStatus UserStatus
	Page       int
	Limit      int
}

// ListedJob is one scored job on a list page.
type ListedJob struct {
	JobID       string
	Title       string
	Company     string
	URL         string
	ApplyURL    string
	Location    jobdata.Location
	Sources     []string
	PostedAt    time.Time
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	Bucket      Bucket
	Score       int
	Signals     []Signal
	UserStatus  UserStatus
}

// JobPage is one page of scored jobs.
type JobPage struct {
	Page  int
	Limit int
	Total int
	Jobs  []ListedJob
}
