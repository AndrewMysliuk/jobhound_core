package schema

// Stage2Rule is one field rule evaluated during stage 2.
type Stage2Rule struct {
	ID             string     `json:"id"`
	Field          RuleField  `json:"field"`
	Op             RuleOp     `json:"op"`
	Values         []string   `json:"values"`
	Action         RuleAction `json:"action"`
	Weight         *int       `json:"weight,omitempty"`
	When           RuleWhen   `json:"when,omitempty"`
	NegationWindow int        `json:"negation_window,omitempty"`
}

// ParsedListing holds normalized fields extracted from a job listing for rule evaluation.
type ParsedListing struct {
	// CountriesAllowed is the union of resolved hiring countries and expanded hiring regions.
	// Empty means worldwide hiring or no restriction stated (both evaluate the same for geo rules).
	CountriesAllowed []string
	Position         string
}
