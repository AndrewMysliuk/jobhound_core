// Package schema holds cross-module vacancy types shared by collectors and jobs storage.
package schema

import "time"

const (
	LocationRemote = "remote"
	LocationHybrid = "hybrid"
	LocationOffice = "office"
)

// Location is the display location stored as one JSON object.
// Only Raw is copied into Job.ID.
type Location struct {
	Type      string
	Regions   []string
	Countries []string
	Timezone  string
	Raw       string
}

// Job is one vacancy. ID is the dedup key set by domain/utils.AssignStableID.
type Job struct {
	ID             string
	Source         string // board that produced this in-memory listing
	Sources        []string
	Title          string
	Company        string
	CompanyKey     string
	CompanyWebsite string
	URL            string
	ApplyURL       string
	Description    string
	PostedAt       time.Time // zero if unknown
	Location       Location
	SalaryRaw      string
	Tags           []string
	Position       *string
	FirstSeenAt    time.Time
	LastSeenAt     time.Time
}
