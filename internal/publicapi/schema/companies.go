package schema

import "time"

// DefaultCompanyListLimit is the default page size for GET /api/v1/companies.
const DefaultCompanyListLimit = 50

// MaxCompanyListLimit is the maximum allowed limit query value.
const MaxCompanyListLimit = 100

// CompanyListItem is one company in GET /api/v1/companies.
// board_etag is not on the wire — internal polling state.
type CompanyListItem struct {
	Key              string     `json:"key"`
	Name             string     `json:"name"`
	Website          string     `json:"website"`
	ATS              string     `json:"ats"`
	ATSSlug          string     `json:"ats_slug"`
	BoardURL         string     `json:"board_url"`
	ResolutionStatus string     `json:"resolution_status"`
	ResolutionMethod string     `json:"resolution_method"`
	PollState        string     `json:"poll_state"`
	LastPolledAt     *time.Time `json:"last_polled_at"`
	PollFailCount    int        `json:"poll_fail_count"`
	NeedsReview      bool       `json:"needs_review"`
}

// CompanyListResponse is the GET /api/v1/companies 200 body.
type CompanyListResponse struct {
	Items []CompanyListItem `json:"items"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
	Total int               `json:"total"`
}
