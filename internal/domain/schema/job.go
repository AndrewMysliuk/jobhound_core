// Package schema holds cross-module vacancy types shared by collectors and jobs storage.
package schema

import "time"

const (
	LocationRemote = "remote"
	LocationHybrid = "hybrid"
	LocationOffice = "office"
)

// Location is the hiring scope used by scoring, stored as one JSON object.
type Location struct {
	Type      string
	Regions   []string
	Countries []string
	Timezone  string
	Raw       string
}

// Opening is one country listing on a vacancy.
// Country is trimmed location raw: display text, not an ISO code.
type Opening struct {
	Country  string
	URL      string
	ApplyURL string
}

// Job is one vacancy. ID is the dedup key set by domain/utils.AssignStableID.
// URL and ApplyURL are the listing being ingested and, after save, the first opening.
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
	Openings       []Opening
	SalaryRaw      string
	Tags           []string
	Position       *string
	FirstSeenAt    time.Time
	LastSeenAt     time.Time
}
