package schema

// Facts is the profile YAML block that turns enrich answers into signals and cuts.
// A missing block, or a missing field, leaves Scored false. A missing Delta is 0.
type Facts struct {
	ResidenceRegions  []string
	Office            FactsOffice
	HiringScope       FactField
	WorkAuthorization FactsWorkAuthorization
	Timezone          FactsTimezone
	Compensation      FactsCompensation
	WorkMode          FactField
	EmploymentLoad    FactField
	Engagement        FactField
}

// FactField is scored/delta for a facts answer with no allow-list or gates.
type FactField struct {
	Scored bool
	Delta  int
}

// FactsOffice is the office allow-lists.
type FactsOffice struct {
	Scored         bool
	Delta          int
	AllowCountries []string
	AllowRegions   []string
}

// FactsWorkAuthorization is the authorization cut list (for example required).
type FactsWorkAuthorization struct {
	Scored bool
	Delta  int
	Gates  []string
}

// FactsTimezone is the allowed timezone zones.
type FactsTimezone struct {
	Scored bool
	Delta  int
	Allow  []string
}

// FactsCompensation is the allowed currencies.
type FactsCompensation struct {
	Scored          bool
	Delta           int
	AllowCurrencies []string
}
