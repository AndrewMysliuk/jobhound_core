package utils

import domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"

// CanonicalListingURL returns the normalized absolute listing URL.
func CanonicalListingURL(raw string) (string, error) {
	return domainutils.NormalizeListingURL(raw)
}
