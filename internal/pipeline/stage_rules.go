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
