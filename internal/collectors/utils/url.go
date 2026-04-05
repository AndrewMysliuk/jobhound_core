package utils

import domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"

// CanonicalListingURL returns the normalized absolute listing URL used before StableJobID
// (specs/001-agent-skeleton-and-domain, specs/005-job-collectors/contracts/collector.md).
func CanonicalListingURL(raw string) (string, error) {
	return domainutils.NormalizeListingURL(raw)
}

// StableJobIDForListing computes the stable job id from source and a raw listing URL after canonicalization.
func StableJobIDForListing(source, rawListingURL string) (string, error) {
	return domainutils.StableJobID(source, rawListingURL)
}
