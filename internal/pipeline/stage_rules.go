package pipeline

import "time"

// BroadFilterRules configures stage 1 (broad filter). Date window and PostedAt comparisons use UTC.
type BroadFilterRules struct {
	From, To *time.Time
	// RoleSynonyms: empty → no role narrowing; otherwise at least one non-empty synonym must appear
	// as a substring in Title or Description (case-insensitive).
	RoleSynonyms []string
	RemoteOnly   bool
	// CountryAllowlist: empty → no country filter; otherwise CountryCode must be known (non-empty)
	// and match one entry (ISO 3166-1 alpha-2, case-insensitive).
	CountryAllowlist []string
}

// KeywordRules configures stage 2 (keyword include/exclude on title + description).
// Matching is case-insensitive; empty patterns after trim are ignored.
type KeywordRules struct {
	// Include: if non-empty after trimming, every non-empty pattern must appear as a substring
	// in the combined title + description text.
	Include []string
	// Exclude: if non-empty, any non-empty pattern appearing in that combined text drops the job.
	Exclude []string
}
