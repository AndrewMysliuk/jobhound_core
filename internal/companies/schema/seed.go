package schema

// SeedRow is one non-header row from companies_template.xlsx (or a fixture).
type SeedRow struct {
	Name         string
	Website      string
	ATSRaw       string // sheet "ATS-тип" as written
	Slug         string // sheet "ATS slug"; never normalized by the importer
	VacanciesURL string // sheet "URL vacancies" (optional board page hint)
	Fit          string // FIT / SKIP
}

// ImportStats summarises one seed import pass.
type ImportStats struct {
	Rows        int // data rows seen (including skipped / blocked)
	Skipped     int // example row or Fit=SKIP
	Blocked     int // blocklisted name
	Created     int
	Merged      int
	Resolved    int // board verify 200 (incl. empty list)
	Pending     int // Custom/UNKNOWN/NONE, or pollable 404 / no slug
	Unsupported int // recognised non-pollable ATS
}
