package vuejobs

import (
	"fmt"
	"net/url"
)

const (
	// SiteOrigin is the VueJobs board origin.
	SiteOrigin = "https://vuejobs.com"
	// DefaultListingPath is the remote catalog listing (no keyword).
	DefaultListingPath = "/jobs"
)

// ListingURL returns the catalog listing URL with work_place=remote and optional page (>1).
func ListingURL(page int) string {
	v := url.Values{}
	v.Set("work_place", "remote")
	if page > 1 {
		v.Set("page", fmt.Sprintf("%d", page))
	}
	return SiteOrigin + DefaultListingPath + "?" + v.Encode()
}

// JobCardURL returns the board listing URL for a job slug.
func JobCardURL(slug string) string {
	return SiteOrigin + DefaultListingPath + "/" + slug
}
