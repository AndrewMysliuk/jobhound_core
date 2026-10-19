package schema

// QuotedValue is one single-value answer. Quote is nil for unknown.
type QuotedValue struct {
	Value string  `json:"value"`
	Quote *string `json:"quote"`
}

// Office is where the company is. Unknown true, with empty code lists,
// is the default when the text names no place. A location badge does not
// fill this by itself. The posting language never fills it.
// Countries are ISO 3166-1 alpha-2 checked strings, not an enum.
type Office struct {
	Countries []string `json:"countries"`
	Regions   []string `json:"regions"`
	Quote     *string  `json:"quote"`
	Unknown   bool     `json:"unknown"`
}

// HiringScope is where the person may work from.
// worldwide only when the text says so. Countries and regions are set
// only when Value is restricted. Unknown is the string "unknown".
// Countries are ISO 3166-1 alpha-2 checked strings, not an enum.
type HiringScope struct {
	Value     string   `json:"value"`
	Countries []string `json:"countries"`
	Regions   []string `json:"regions"`
	Quote     *string  `json:"quote"`
}

// WorkAuthorization is an existing right-to-work requirement.
// none only when the text says an existing right is not required.
// Countries may be empty when the text requires it and names no country.
// Countries are ISO 3166-1 alpha-2 checked strings, not an enum.
type WorkAuthorization struct {
	Value     string   `json:"value"`
	Countries []string `json:"countries"`
	Quote     *string  `json:"quote"`
}

type TimezoneConstraint struct {
	Value string   `json:"value"`
	Zones []string `json:"zones"`
	Quote *string  `json:"quote"`
}

// Compensation is the offered pay. Unknown true means no currency was
// stated, or the quote check failed. Empty Currency is not a second answer.
// Currency is an ISO 4217 checked string, not an enum. A currency does not
// fill Office. Min and max stay nil when no amount is stated.
type Compensation struct {
	Currency string   `json:"currency"`
	Min      *float64 `json:"min"`
	Max      *float64 `json:"max"`
	Period   string   `json:"period"`
	Quote    *string  `json:"quote"`
	Unknown  bool     `json:"unknown"`
}

// ModelReply is the batch JSON. Unknown on a scope or a quoted value is
// the string "unknown". Unknown on office and compensation is the bool.
type ModelReply struct {
	Office            Office             `json:"office"`
	HiringScope       HiringScope        `json:"hiring_scope"`
	WorkAuthorization WorkAuthorization  `json:"work_authorization"`
	Timezone          TimezoneConstraint `json:"timezone"`
	Compensation      Compensation       `json:"compensation"`
	WorkMode          QuotedValue        `json:"work_mode"`
	EmploymentLoad    QuotedValue        `json:"employment_load"`
	Engagement        QuotedValue        `json:"engagement"`
}
