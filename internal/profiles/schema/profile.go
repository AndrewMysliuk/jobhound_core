package schema

// Profile is one search definition loaded from YAML.
type Profile struct {
	ID             string
	Name           string
	Domain         Domain
	Sources        []string
	Queries        []string
	Children       map[string][]string
	WellfoundRoles []string
	GateRole       []string
	GateHard       []string
	GateLocation   GateLocation
	SignalsPlus    SignalsPlus
	PenaltiesSoft  []PenaltyGroup
	Buckets        Buckets
	Facts          Facts
}

// GateLocation is the allow lists and the rejected timezone phrases.
type GateLocation struct {
	AllowCountries  []string
	AllowRegions    []string
	RejectTimezones []string
}

// SignalsPlus is the positive score block. Weights come from the profile.
type SignalsPlus struct {
	Title     map[string]int
	Text      TextSignals
	Salary    SalarySignals
	Freshness FreshnessSignals
}

// TextSignals is the body term map and its cap.
type TextSignals struct {
	Terms    map[string]int
	MaxTotal int
}

// SalarySignals compares a parsed upper bound with the profile minimums.
type SalarySignals struct {
	MinEURYear     int
	MinUSDYear     int
	USDPerEUR      float64
	BonusIfAbove   int
	PenaltyIfBelow int
}

// FreshnessSignals is the posted-at bonus.
type FreshnessSignals struct {
	Days3 int
	Days7 int
}

// PenaltyGroup is one soft penalty, visited in slice order.
type PenaltyGroup struct {
	Name           string
	Weight         int
	MinOccurrences int
	CheckNegation  bool
	Terms          []string
}

// Buckets holds the score cutoff for TOP.
type Buckets struct {
	Top int
}
