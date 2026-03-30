package europeremotely

import "net/url"

// Default endpoints for euremotejobs.com (specs/005-job-collectors/resources/europe-remotely.md).
const (
	DefaultSiteBaseURL = "https://euremotejobs.com/"
	DefaultFeedURL     = "https://euremotejobs.com/wp-admin/admin-ajax.php"
)

// DefaultSiteBase parses DefaultSiteBaseURL for resolving relative links in listing/detail HTML.
func DefaultSiteBase() (*url.URL, error) {
	return url.Parse(DefaultSiteBaseURL)
}
