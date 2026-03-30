package europeremotely

// CSS selectors for Europe Remotely markup (AJAX listing fragments + full job pages).
// Update here when the site theme changes; parsing logic stays in parse.go.

const (
	selListingRoot      = ".job-card"
	selListingTitleLink = "h2.job-title a"
	selListingTitle     = "h2.job-title"
	selListingCompany   = "div.company-name"
	selListingLocation  = "div.meta-item.meta-location"
	selListingPosted    = "div.job-time"
	selListingMetaItem  = "div.meta-item"
	selListingCardWrap  = "a.job-card-link"
)

const (
	selDetailTitle       = "h1.page-title"
	selDetailCompany     = "ul.job-listing-meta li.job-company"
	selDetailLocLink     = "ul.job-listing-meta li.location a.google_map_link"
	selDetailLocation    = "ul.job-listing-meta li.location"
	selDetailDatePosted  = "ul.job-listing-meta li.date-posted"
	selDetailSalary      = "ul.job-listing-meta li.wpjmef-field-salary"
	selDetailDescription = "div.job_listing-description"
	selDetailApply       = "a.application_button_link"
	selDetailTags        = "p.job_tags"
)
