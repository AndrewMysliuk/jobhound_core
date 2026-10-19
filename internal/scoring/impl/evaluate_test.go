package impl

import (
	"reflect"
	"strings"
	"testing"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	enrichschema "github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

func TestEvaluate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	month := time.Duration(schema.PostedAtRejectDays) * 24 * time.Hour
	stale := now.Add(-month).Add(-time.Second)
	review := profileschema.Buckets{Top: 1}
	signal := func(code schema.SignalCode, rule, phrase string, occurrences int) schema.Signal {
		return schema.Signal{Code: code, Rule: rule, Phrase: phrase, Occurrences: occurrences}
	}
	role := func(phrase string, occurrences int) schema.Signal {
		return signal(schema.SignalGateRole, "gate_role", phrase, occurrences)
	}
	hard := func(phrase string, occurrences int) schema.Signal {
		return signal(schema.SignalGateHard, "gate_hard", phrase, occurrences)
	}
	europe := func(code string) schema.Signal {
		return signal(schema.SignalCutLocation, "gate_location.europe", code, 0)
	}
	europeOffice := schema.Signal{Code: schema.SignalPenalty, Rule: "europe_office", Delta: -2}
	region := func(code string) schema.Signal {
		return signal(schema.SignalCutLocation, "gate_location.allow_regions", code, 0)
	}
	timezone := func(phrase string, occurrences int) schema.Signal {
		return signal(schema.SignalCutTimezone, "gate_location.reject_timezones", phrase, occurrences)
	}
	plus := func(rule, phrase string, delta, occurrences int) schema.Signal {
		return schema.Signal{Code: schema.SignalPlus, Rule: rule, Phrase: phrase, Delta: delta, Occurrences: occurrences}
	}
	penalty := func(rule, phrase string, delta, occurrences int) schema.Signal {
		return schema.Signal{Code: schema.SignalPenalty, Rule: rule, Phrase: phrase, Delta: delta, Occurrences: occurrences}
	}
	soft := func(name string, weight, min int, check bool, terms ...string) profileschema.PenaltyGroup {
		return profileschema.PenaltyGroup{
			Name: name, Weight: weight, MinOccurrences: min, CheckNegation: check, Terms: terms,
		}
	}
	day := 24 * time.Hour
	locations := profileschema.Profile{
		GateLocation: profileschema.GateLocation{
			AllowCountries: []string{"US", "DE", "RO", "GB"},
			AllowRegions:   []string{"WORLDWIDE", "EUROPE", "EMEA"},
		},
		Buckets: review,
	}

	tests := []struct {
		name    string
		profile profileschema.Profile
		job     jobdata.Job
		want    Outcome
	}{
		{
			name:    "not part-time does not count",
			profile: profileschema.Profile{GateHard: []string{"part-time"}, Buckets: review},
			job:     jobdata.Job{Description: "not part-time"},
			want:    Outcome{Bucket: schema.BucketReview},
		},
		{
			name:    "no US authorization required does not count",
			profile: profileschema.Profile{GateHard: []string{"US authorization required"}, Buckets: review},
			job:     jobdata.Job{Description: "no US authorization required"},
			want:    Outcome{Bucket: schema.BucketReview},
		},
		{
			name:    "you won't need to relocate does not count",
			profile: profileschema.Profile{GateHard: []string{"relocate"}, Buckets: review},
			job:     jobdata.Job{Description: "you won't need to relocate"},
			want:    Outcome{Bucket: schema.BucketReview},
		},
		{
			name:    "non-hybrid does not count",
			profile: profileschema.Profile{GateHard: []string{"hybrid"}, Buckets: review},
			job:     jobdata.Job{Description: "non-hybrid"},
			want:    Outcome{Bucket: schema.BucketReview},
		},
		{
			name:    "negation in the previous sentence still rejects",
			profile: profileschema.Profile{GateHard: []string{"security clearance"}, Buckets: review},
			job:     jobdata.Job{Description: "We are not a huge company. Security clearance required."},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{hard("security clearance", 1)},
			},
		},
		{
			name: "negation more than 50 characters away still rejects",
			profile: profileschema.Profile{
				GateHard: []string{"part-time"},
			},
			job: jobdata.Job{Description: "not " + strings.Repeat("x", 50) + " part-time"},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{hard("part-time", 1)},
			},
		},
		{
			name: "gate role is title only and ignores negation",
			profile: profileschema.Profile{
				GateRole: []string{"recruiter"},
			},
			job: jobdata.Job{
				Title:       "not Recruiter recruiter",
				Description: "a recruiter will call",
			},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{role("recruiter", 2)},
			},
		},
		{
			name: "gate role ignores the description",
			profile: profileschema.Profile{
				GateRole: []string{"recruiter"},
				Buckets:  review,
			},
			job:  jobdata.Job{Title: "Engineer", Description: "a recruiter will call"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "hard gate counts title description and the location line",
			profile: profileschema.Profile{
				GateHard: []string{"clearance"},
			},
			job: jobdata.Job{
				Title:       "clearance",
				Description: "clearance",
				Location:    jobdata.Location{Raw: "clearance", Timezone: "clearance"},
			},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{hard("clearance", 3)},
			},
		},
		{
			name: "java does not match javascript",
			profile: profileschema.Profile{
				GateHard: []string{"java"},
				Buckets:  review,
			},
			job:  jobdata.Job{Description: "We use javascript"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "java matches a bounded word",
			profile: profileschema.Profile{
				GateHard: []string{"java"},
			},
			job: jobdata.Job{Description: "We use Java."},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{hard("java", 1)},
			},
		},
		{
			name:    "empty location is not rejected",
			profile: locations,
			job:     jobdata.Job{Title: "Frontend Engineer"},
			want:    Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "empty allow lists are not a reject",
			profile: profileschema.Profile{
				Buckets: review,
			},
			job: jobdata.Job{
				Location: jobdata.Location{Countries: []string{"RO"}, Regions: []string{"LATAM"}},
			},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name:    "allowed country stays",
			profile: locations,
			job:     jobdata.Job{Location: jobdata.Location{Countries: []string{"RO"}}},
			want:    Outcome{Bucket: schema.BucketReview},
		},
		{
			name:    "allowed region stays",
			profile: locations,
			job:     jobdata.Job{Location: jobdata.Location{Regions: []string{"emea"}}},
			want:    Outcome{Bucket: schema.BucketReview},
		},
		{
			name:    "disallowed codes are one cut each",
			profile: locations,
			job: jobdata.Job{
				Location: jobdata.Location{Countries: []string{"BR", "AR"}, Regions: []string{"LATAM"}},
			},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{europe("BR"), europe("AR"), region("LATAM")},
			},
		},
		{
			name:    "a disallowed country is a cut when another country is allowed",
			profile: locations,
			job:     jobdata.Job{Location: jobdata.Location{Countries: []string{"DE", "BR"}}},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{europe("BR")},
			},
		},
		{
			name: "remote in the US is not a location cut",
			profile: profileschema.Profile{
				GateLocation: profileschema.GateLocation{AllowCountries: []string{"RO"}},
				Buckets:      review,
			},
			job: jobdata.Job{Location: jobdata.Location{
				Type:      jobdata.LocationRemote,
				Countries: []string{"US"},
			}},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name:    "office in DE scores the europe penalty once",
			profile: profileschema.Profile{Buckets: review},
			job: jobdata.Job{Location: jobdata.Location{
				Type:      jobdata.LocationOffice,
				Countries: []string{"DE"},
			}},
			want: Outcome{
				Bucket:  schema.BucketReview,
				Score:   -2,
				Signals: []schema.Signal{europeOffice},
			},
		},
		{
			name:    "office in DE with umzug has no europe penalty",
			profile: profileschema.Profile{Buckets: review},
			job: jobdata.Job{
				Description: "umzug",
				Location: jobdata.Location{
					Type:      jobdata.LocationOffice,
					Countries: []string{"DE"},
				},
			},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name:    "office in RO has no europe penalty",
			profile: profileschema.Profile{Buckets: review},
			job: jobdata.Job{Location: jobdata.Location{
				Type:      jobdata.LocationOffice,
				Countries: []string{"RO"},
			}},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "office in the US is a location cut",
			profile: profileschema.Profile{
				GateLocation: profileschema.GateLocation{AllowCountries: []string{"US"}},
				Buckets:      review,
			},
			job: jobdata.Job{Location: jobdata.Location{
				Type:      jobdata.LocationOffice,
				Countries: []string{"US"},
			}},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{europe("US")},
			},
		},
		{
			name: "timezone phrase",
			profile: profileschema.Profile{
				GateLocation: profileschema.GateLocation{RejectTimezones: []string{"EST"}},
			},
			job: jobdata.Job{Location: jobdata.Location{Timezone: "EST"}},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{timezone("EST", 1)},
			},
		},
		{
			name: "negated timezone does not count",
			profile: profileschema.Profile{
				GateLocation: profileschema.GateLocation{RejectTimezones: []string{"EST"}},
				Buckets:      review,
			},
			job:  jobdata.Job{Description: "you won't need EST hours"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "zero posted_at is not a reject",
			profile: profileschema.Profile{
				Buckets: review,
			},
			job:  jobdata.Job{Title: "Engineer"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "posted exactly at the cutoff is not a reject",
			profile: profileschema.Profile{
				Buckets: review,
			},
			job:  jobdata.Job{PostedAt: now.Add(-month)},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "stale posted_at is rejected",
			profile: profileschema.Profile{
				Buckets: profileschema.Buckets{Top: 0},
			},
			job: jobdata.Job{PostedAt: stale},
			want: Outcome{
				Bucket: schema.BucketRejected,
				Signals: []schema.Signal{{
					Code:   schema.SignalCutPostedAt,
					Rule:   "posted_at",
					Phrase: stale.UTC().Format(time.RFC3339),
				}},
			},
		},
		{
			name: "score at the top cutoff is top",
			profile: profileschema.Profile{
				Buckets: profileschema.Buckets{Top: 0},
			},
			job:  jobdata.Job{Title: "Engineer"},
			want: Outcome{Bucket: schema.BucketTop},
		},
		{
			name: "every gate is recorded",
			profile: profileschema.Profile{
				GateRole: []string{"recruiter"},
				GateHard: []string{"security clearance"},
				GateLocation: profileschema.GateLocation{
					AllowCountries:  []string{"DE"},
					RejectTimezones: []string{"EST"},
				},
				Buckets: profileschema.Buckets{Top: 0},
			},
			job: jobdata.Job{
				Title:       "Recruiter",
				Description: "security clearance EST",
				Location:    jobdata.Location{Countries: []string{"BR"}},
				PostedAt:    stale,
			},
			want: Outcome{
				Bucket: schema.BucketRejected,
				Signals: []schema.Signal{
					role("recruiter", 1),
					hard("security clearance", 1),
					europe("BR"),
					timezone("EST", 1),
					{
						Code:   schema.SignalCutPostedAt,
						Rule:   "posted_at",
						Phrase: stale.UTC().Format(time.RFC3339),
					},
				},
			},
		},
		{
			name: "title terms keep the key and walk in sorted order",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{Title: map[string]int{"Vue": 3, "go": 2}},
				Buckets:     review,
			},
			job: jobdata.Job{Title: "go Vue"},
			want: Outcome{
				Bucket: schema.BucketTop,
				Score:  5,
				Signals: []schema.Signal{
					plus("signals_plus.title", "Vue", 3, 1),
					plus("signals_plus.title", "go", 2, 1),
				},
			},
		},
		{
			name: "title weight is awarded once",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{Title: map[string]int{"go": 2}},
				Buckets:     review,
			},
			job: jobdata.Job{Title: "Go go", Description: "go"},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   2,
				Signals: []schema.Signal{plus("signals_plus.title", "go", 2, 2)},
			},
		},
		{
			name: "go does not match golang",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{Title: map[string]int{"go": 2}},
				Buckets:     review,
			},
			job:  jobdata.Job{Title: "Golang Developer"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "body terms do not read the title",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Text: profileschema.TextSignals{Terms: map[string]int{"vue": 3}, MaxTotal: 10},
				},
				Buckets: review,
			},
			job:  jobdata.Job{Title: "Vue"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "body cap awards the points that fit and then zero",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Text: profileschema.TextSignals{
						Terms:    map[string]int{"go": 4, "rust": 4, "vue": 3},
						MaxTotal: 5,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{Description: "We use Go, Rust, and Vue."},
			want: Outcome{
				Bucket: schema.BucketTop,
				Score:  5,
				Signals: []schema.Signal{
					plus("signals_plus.text", "go", 4, 1),
					plus("signals_plus.text", "rust", 1, 1),
					plus("signals_plus.text", "vue", 0, 1),
				},
			},
		},
		{
			name: "a spent body cap still emits the term",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Text: profileschema.TextSignals{Terms: map[string]int{"go": 4}, MaxTotal: 0},
				},
				Buckets: review,
			},
			job: jobdata.Job{Description: "Go"},
			want: Outcome{
				Bucket:  schema.BucketReview,
				Signals: []schema.Signal{plus("signals_plus.text", "go", 0, 1)},
			},
		},
		{
			name: "plus terms ignore negation",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Text: profileschema.TextSignals{Terms: map[string]int{"vue": 4}, MaxTotal: 10},
				},
				Buckets: review,
			},
			job: jobdata.Job{Description: "not vue"},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   4,
				Signals: []schema.Signal{plus("signals_plus.text", "vue", 4, 1)},
			},
		},
		{
			name: "unset salary does not score a parsed amount",
			profile: profileschema.Profile{
				Buckets: review,
			},
			job:  jobdata.Job{SalaryRaw: "€70k – €90k"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "missing salary does not apply the below penalty",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 80000, MinUSDYear: 90000, USDPerEUR: 1.1,
						BonusIfAbove: 6, PenaltyIfBelow: -9,
					},
				},
				Buckets: review,
			},
			job:  jobdata.Job{Title: "Engineer", Description: "No compensation listed"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "unparseable salary does not apply the below penalty",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 80000, BonusIfAbove: 6, PenaltyIfBelow: -9,
					},
				},
				Buckets: review,
			},
			job:  jobdata.Job{SalaryRaw: "competitive", Description: "paid well"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "eur upper bound at the minimum earns the bonus",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 90000, MinUSDYear: 200000, USDPerEUR: 2,
						BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{SalaryRaw: "€70k – €90k"},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   6,
				Signals: []schema.Signal{plus("signals_plus.salary", "", 6, 0)},
			},
		},
		{
			name: "yearly eur below the minimum keeps the stored penalty",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 200000, BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{SalaryRaw: "90,000–110,000 EUR/year"},
			want: Outcome{
				Bucket:  schema.BucketReview,
				Score:   -4,
				Signals: []schema.Signal{plus("signals_plus.salary", "", -4, 0)},
			},
		},
		{
			name: "usd salary uses the usd minimum and the upper bound",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 200000, MinUSDYear: 150000, USDPerEUR: 2,
						BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{SalaryRaw: "120000-180000 USD"},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   6,
				Signals: []schema.Signal{plus("signals_plus.salary", "", 6, 0)},
			},
		},
		{
			name: "usd salary converts through usd per eur when the usd minimum is unset",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 60000, USDPerEUR: 2,
						BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{SalaryRaw: "USD 100000"},
			want: Outcome{
				Bucket:  schema.BucketReview,
				Score:   -4,
				Signals: []schema.Signal{plus("signals_plus.salary", "", -4, 0)},
			},
		},
		{
			name: "one sided eur salary uses that bound",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 80000, BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{SalaryRaw: "100000- EUR"},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   6,
				Signals: []schema.Signal{plus("signals_plus.salary", "", 6, 0)},
			},
		},
		{
			name: "monthly eur is annualized",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 50000, BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{Description: "5,000 EUR/month"},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   6,
				Signals: []schema.Signal{plus("signals_plus.salary", "", 6, 0)},
			},
		},
		{
			name: "hourly usd is annualized at a full-time year",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinUSDYear: 60 * fullTimeHoursPerYear, BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{Title: "$60/hour"},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   6,
				Signals: []schema.Signal{plus("signals_plus.salary", "", 6, 0)},
			},
		},
		{
			name: "salary raw wins over a description amount",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 80000, MinUSDYear: 200000, USDPerEUR: 2,
						BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{SalaryRaw: "€90k", Description: "$5,000 signing"},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   6,
				Signals: []schema.Signal{plus("signals_plus.salary", "", 6, 0)},
			},
		},
		{
			name: "title salary is used when the raw field is empty",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Salary: profileschema.SalarySignals{
						MinEURYear: 80000, MinUSDYear: 200000, USDPerEUR: 2,
						BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
				},
				Buckets: review,
			},
			job: jobdata.Job{Title: "Frontend €90k", Description: "$5,000 signing"},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   6,
				Signals: []schema.Signal{plus("signals_plus.salary", "", 6, 0)},
			},
		},
		{
			name: "posted within 3 days adds days 3",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{Freshness: profileschema.FreshnessSignals{Days3: 4, Days7: 1}},
				Buckets:     review,
			},
			job: jobdata.Job{PostedAt: now.Add(-3 * day), FirstSeenAt: now.Add(-8 * day)},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   4,
				Signals: []schema.Signal{plus("signals_plus.freshness", "", 4, 0)},
			},
		},
		{
			name: "posted within 7 days adds days 7",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{Freshness: profileschema.FreshnessSignals{Days3: 4, Days7: 1}},
				Buckets:     review,
			},
			job: jobdata.Job{PostedAt: now.Add(-7 * day)},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   1,
				Signals: []schema.Signal{plus("signals_plus.freshness", "", 1, 0)},
			},
		},
		{
			name: "zero freshness awards no signal",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{Freshness: profileschema.FreshnessSignals{}},
				Buckets:     review,
			},
			job:  jobdata.Job{PostedAt: now.Add(-2 * day)},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "posted older than 7 days adds no freshness",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{Freshness: profileschema.FreshnessSignals{Days3: 4, Days7: 1}},
				Buckets:     review,
			},
			job:  jobdata.Job{PostedAt: now.Add(-7*day - time.Second), FirstSeenAt: now.Add(-day)},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "freshness falls back to first seen",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{Freshness: profileschema.FreshnessSignals{Days3: 4, Days7: 1}},
				Buckets:     review,
			},
			job: jobdata.Job{FirstSeenAt: now.Add(-2 * day)},
			want: Outcome{
				Bucket:  schema.BucketTop,
				Score:   4,
				Signals: []schema.Signal{plus("signals_plus.freshness", "", 4, 0)},
			},
		},
		{
			name: "plus signals run title text salary freshness",
			profile: profileschema.Profile{
				SignalsPlus: profileschema.SignalsPlus{
					Title: map[string]int{"go": 8},
					Text:  profileschema.TextSignals{Terms: map[string]int{"vue": 5}, MaxTotal: 3},
					Salary: profileschema.SalarySignals{
						MinEURYear: 80000, BonusIfAbove: 6, PenaltyIfBelow: -4,
					},
					Freshness: profileschema.FreshnessSignals{Days3: 4, Days7: 1},
				},
				Buckets: review,
			},
			job: jobdata.Job{
				Title:       "Go",
				Description: "Vue",
				SalaryRaw:   "€90k",
				PostedAt:    now.Add(-2 * day),
			},
			want: Outcome{
				Bucket: schema.BucketTop,
				Score:  21,
				Signals: []schema.Signal{
					plus("signals_plus.title", "go", 8, 1),
					plus("signals_plus.text", "vue", 3, 1),
					plus("signals_plus.salary", "", 6, 0),
					plus("signals_plus.freshness", "", 4, 0),
				},
			},
		},
		{
			name: "one ruby in the body scores nothing",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{soft("foreign_stack", -3, 2, false, "ruby")},
				Buckets:       review,
			},
			job:  jobdata.Job{Description: "We use Ruby."},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "two ruby hits apply the weight once",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{soft("foreign_stack", -3, 2, false, "ruby")},
				Buckets:       review,
			},
			job: jobdata.Job{Description: "Ruby and ruby"},
			want: Outcome{
				Bucket:  schema.BucketReview,
				Score:   -3,
				Signals: []schema.Signal{penalty("foreign_stack", "ruby", -3, 2)},
			},
		},
		{
			name: "ruby in the title applies the weight once",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{soft("foreign_stack", -3, 2, false, "ruby")},
				Buckets:       review,
			},
			job: jobdata.Job{Title: "Ruby Ruby Ruby", Description: "ruby"},
			want: Outcome{
				Bucket:  schema.BucketReview,
				Score:   -3,
				Signals: []schema.Signal{penalty("foreign_stack", "ruby", -3, 3)},
			},
		},
		{
			name: "penalty min occurrences of zero means one body hit",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{soft("office_pressure", -2, 0, false, "hybrid")},
				Buckets:       review,
			},
			job: jobdata.Job{Description: "Hybrid"},
			want: Outcome{
				Bucket:  schema.BucketReview,
				Score:   -2,
				Signals: []schema.Signal{penalty("office_pressure", "hybrid", -2, 1)},
			},
		},
		{
			name: "a negated penalty term does not count when the group asks",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{soft("part_time", -4, 1, true, "part-time")},
				Buckets:       review,
			},
			job:  jobdata.Job{Description: "not part-time"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "a negated penalty term counts when the group does not ask",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{soft("foreign_stack", -3, 1, false, "ruby")},
				Buckets:       review,
			},
			job: jobdata.Job{Description: "not ruby"},
			want: Outcome{
				Bucket:  schema.BucketReview,
				Score:   -3,
				Signals: []schema.Signal{penalty("foreign_stack", "ruby", -3, 1)},
			},
		},
		{
			name: "the first qualifying term fires the group once",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{soft("foreign_stack", -3, 2, false, "ruby", "php")},
				Buckets:       review,
			},
			job: jobdata.Job{Description: "php php and ruby"},
			want: Outcome{
				Bucket:  schema.BucketReview,
				Score:   -3,
				Signals: []schema.Signal{penalty("foreign_stack", "php", -3, 2)},
			},
		},
		{
			name: "penalty groups stay in slice order",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{
					soft("agency", -2, 1, false, "recruiting"),
					soft("foreign_stack", -3, 2, false, "ruby"),
				},
				Buckets: review,
			},
			job: jobdata.Job{Title: "Recruiting", Description: "ruby ruby"},
			want: Outcome{
				Bucket: schema.BucketReview,
				Score:  -5,
				Signals: []schema.Signal{
					penalty("agency", "recruiting", -2, 1),
					penalty("foreign_stack", "ruby", -3, 2),
				},
			},
		},
		{
			name: "junior or senior does not fire the level gates",
			profile: profileschema.Profile{
				GateRole: []string{"senior", "sr.", "lead", "principal"},
				Buckets:  review,
			},
			job:  jobdata.Job{Title: "Landscape Architect (Junior or Senior)"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "junior/senior/lead does not fire the level gates",
			profile: profileschema.Profile{
				GateRole: []string{"senior", "sr.", "lead", "principal"},
				Buckets:  review,
			},
			job:  jobdata.Job{Title: "Landscape Architect (Junior/Senior/Lead)"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "jr. or sr. does not fire the level gates",
			profile: profileschema.Profile{
				GateRole: []string{"senior", "sr.", "lead", "principal"},
				Buckets:  review,
			},
			job:  jobdata.Job{Title: "Landscape Architect (Jr. or Sr.)"},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "a senior title still rejects",
			profile: profileschema.Profile{
				GateRole: []string{"senior", "lead", "principal"},
				Buckets:  review,
			},
			job: jobdata.Job{Title: "Senior Landscape Architect"},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{role("senior", 1)},
			},
		},
		{
			name: "senior or lead without junior still rejects",
			profile: profileschema.Profile{
				GateRole: []string{"senior", "lead", "principal"},
				Buckets:  review,
			},
			job: jobdata.Job{Title: "Senior or Lead Architect"},
			want: Outcome{
				Bucket: schema.BucketRejected,
				Signals: []schema.Signal{
					role("senior", 1),
					role("lead", 1),
				},
			},
		},
		{
			name: "senior outside a junior disjunction still rejects",
			profile: profileschema.Profile{
				GateRole: []string{"senior", "lead", "principal"},
				Buckets:  review,
			},
			job: jobdata.Job{Title: "Senior Architect (Junior or Senior)"},
			want: Outcome{
				Bucket:  schema.BucketRejected,
				Signals: []schema.Signal{role("senior", 1)},
			},
		},
		{
			name: "deutschkenntnisse is gate hard",
			profile: profileschema.Profile{
				GateHard: []string{"deutschkenntnisse", "gute deutschkenntnisse"},
				Buckets:  review,
			},
			job: jobdata.Job{Description: "Wir erwarten sehr gute Deutschkenntnisse."},
			want: Outcome{
				Bucket: schema.BucketRejected,
				Signals: []schema.Signal{
					hard("deutschkenntnisse", 1),
					hard("gute deutschkenntnisse", 1),
				},
			},
		},
		{
			name: "german diacritics fold for gate hard",
			profile: profileschema.Profile{
				GateHard: []string{"spate munchner grosse"},
				Buckets:  review,
			},
			job: jobdata.Job{Description: "Späte Münchner Größe"},
			want: Outcome{
				Bucket: schema.BucketRejected,
				Signals: []schema.Signal{
					hard("spate munchner grosse", 1),
				},
			},
		},
		{
			name: "a gate role with a high score stays rejected",
			profile: profileschema.Profile{
				GateRole:    []string{"recruiter"},
				SignalsPlus: profileschema.SignalsPlus{Title: map[string]int{"vue": 5}},
				Buckets:     review,
			},
			job: jobdata.Job{Title: "Vue Recruiter"},
			want: Outcome{
				Bucket: schema.BucketRejected,
				Score:  5,
				Signals: []schema.Signal{
					role("recruiter", 1),
					plus("signals_plus.title", "vue", 5, 1),
				},
			},
		},
		{
			name: "architecture body-only score stays review",
			profile: profileschema.Profile{
				Domain: profileschema.DomainArchitecture,
				SignalsPlus: profileschema.SignalsPlus{
					Text: profileschema.TextSignals{
						MaxTotal: 4,
						Terms:    map[string]int{"revit": 2, "autocad": 2, "proiectare": 1},
					},
				},
				Buckets: profileschema.Buckets{Top: 6},
			},
			job: jobdata.Job{Title: "BIM Manager", Description: "revit autocad proiectare"},
			want: Outcome{
				Bucket: schema.BucketReview,
				Score:  4,
				Signals: []schema.Signal{
					plus("signals_plus.text", "autocad", 2, 1),
					plus("signals_plus.text", "proiectare", 1, 1),
					plus("signals_plus.text", "revit", 1, 1),
				},
			},
		},
		{
			name: "architecture city terms match location raw with diacritic fold",
			profile: profileschema.Profile{
				Domain: profileschema.DomainArchitecture,
				SignalsPlus: profileschema.SignalsPlus{
					Text: profileschema.TextSignals{
						MaxTotal: 4,
						Terms:    map[string]int{"brasov": 4},
					},
				},
				Buckets: profileschema.Buckets{Top: 6},
			},
			job: jobdata.Job{
				Title:    "Arhitect",
				Location: jobdata.Location{Raw: "Strada Gheorghe Lazăr, Brașov, Romania"},
			},
			want: Outcome{
				Bucket: schema.BucketReview,
				Score:  4,
				Signals: []schema.Signal{
					plus("signals_plus.text", "brasov", 4, 1),
				},
			},
		},
		{
			name: "architecture experience over three years is gate hard",
			profile: profileschema.Profile{
				Domain:  profileschema.DomainArchitecture,
				Buckets: review,
			},
			job: jobdata.Job{Description: "Minimum 5 years in Revit."},
			want: Outcome{
				Bucket: schema.BucketRejected,
				Signals: []schema.Signal{{
					Code:        schema.SignalGateHard,
					Rule:        "experience_years",
					Phrase:      "5",
					Occurrences: 1,
				}},
			},
		},
		{
			name: "software profile ignores experience years gate",
			profile: profileschema.Profile{
				Domain:  profileschema.DomainSoftware,
				Buckets: review,
			},
			job:  jobdata.Job{Description: "Minimum 10 years of Go."},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "wrong domain penalty needs two distinct terms",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{
					soft("wrong_domain", -6, 2, false, "hvac", "devops", "revit"),
				},
				Buckets: review,
			},
			job:  jobdata.Job{Description: "HVAC coordination on site."},
			want: Outcome{Bucket: schema.BucketReview},
		},
		{
			name: "wrong domain penalty fires on two terms",
			profile: profileschema.Profile{
				PenaltiesSoft: []profileschema.PenaltyGroup{
					soft("wrong_domain", -6, 2, false, "hvac", "devops", "revit"),
				},
				Buckets: review,
			},
			job: jobdata.Job{Description: "HVAC and DevOps pipeline."},
			want: Outcome{
				Bucket: schema.BucketReview,
				Score:  -6,
				Signals: []schema.Signal{
					penalty("wrong_domain", "hvac", -6, 2),
				},
			},
		},
		{
			name: "architecture stale posted_at uses ninety day cutoff",
			profile: profileschema.Profile{
				Domain:  profileschema.DomainArchitecture,
				Buckets: review,
			},
			job: jobdata.Job{
				PostedAt: now.Add(-time.Duration(schema.PostedAtRejectDaysArchitecture) * 24 * time.Hour).Add(-time.Second),
			},
			want: Outcome{
				Bucket: schema.BucketRejected,
				Signals: []schema.Signal{{
					Code:   schema.SignalCutPostedAt,
					Rule:   "posted_at",
					Phrase: now.Add(-time.Duration(schema.PostedAtRejectDaysArchitecture) * 24 * time.Hour).Add(-time.Second).Format(time.RFC3339),
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := evaluateAt(tt.profile, tt.job, now, nil)
			if got.Bucket != tt.want.Bucket || got.Score != tt.want.Score || !reflect.DeepEqual(got.Signals, tt.want.Signals) {
				t.Fatalf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestFactSignals(t *testing.T) {
	t.Parallel()

	quote := func(s string) *string { return &s }
	review := profileschema.Buckets{Top: 1}
	baseFacts := profileschema.Facts{
		ResidenceRegions: []string{"EUROPE"},
		Office: profileschema.FactsOffice{
			AllowCountries: []string{"US", "DE", "FR", "RO", "GB"},
			AllowRegions:   []string{"EUROPE", "EMEA", "WORLDWIDE"},
		},
		WorkAuthorization: profileschema.FactsWorkAuthorization{Gates: []string{"required"}},
		Timezone:          profileschema.FactsTimezone{Allow: []string{"CET", "GMT", "EET"}},
		Compensation:      profileschema.FactsCompensation{AllowCurrencies: []string{"EUR", "USD", "GBP"}},
	}

	tests := []struct {
		name    string
		profile profileschema.Profile
		job     jobdata.Job
		verdict *enrichschema.Verdict
		want    []schema.Signal
	}{
		{
			name:    "nil verdict adds no fact signals",
			profile: profileschema.Profile{Facts: baseFacts, Buckets: review},
			job:     jobdata.Job{Description: "Remote in Europe"},
			verdict: nil,
			want:    nil,
		},
		{
			name:    "unknown fields emit nothing",
			profile: profileschema.Profile{Facts: baseFacts, Buckets: review},
			job:     jobdata.Job{Description: "A role"},
			verdict: &enrichschema.Verdict{Facts: enrichschema.ModelReply{
				Office:            enrichschema.Office{Unknown: true},
				HiringScope:       enrichschema.HiringScope{Value: "unknown"},
				WorkAuthorization: enrichschema.WorkAuthorization{Value: "unknown"},
				Timezone:          enrichschema.TimezoneConstraint{Value: "unknown"},
				Compensation:      enrichschema.Compensation{Unknown: true},
				WorkMode:          enrichschema.QuotedValue{Value: "unknown"},
				EmploymentLoad:    enrichschema.QuotedValue{Value: "unknown"},
				Engagement:        enrichschema.QuotedValue{Value: "unknown"},
			}},
			want: nil,
		},
		{
			name: "scored false emits LLM at delta 0",
			profile: profileschema.Profile{
				Facts:   baseFacts,
				Buckets: review,
			},
			job: jobdata.Job{Description: "Company in Berlin"},
			verdict: &enrichschema.Verdict{Facts: enrichschema.ModelReply{
				Office: enrichschema.Office{
					Countries: []string{"DE"},
					Quote:     quote("Berlin"),
				},
				HiringScope:       enrichschema.HiringScope{Value: "unknown"},
				WorkAuthorization: enrichschema.WorkAuthorization{Value: "unknown"},
				Timezone:          enrichschema.TimezoneConstraint{Value: "unknown"},
				Compensation:      enrichschema.Compensation{Unknown: true},
				WorkMode:          enrichschema.QuotedValue{Value: "unknown"},
				EmploymentLoad:    enrichschema.QuotedValue{Value: "unknown"},
				Engagement:        enrichschema.QuotedValue{Value: "unknown"},
			}},
			want: []schema.Signal{{
				Code:   schema.SignalLLM,
				Rule:   "office",
				Phrase: "Berlin",
				Value:  "DE",
				Delta:  0,
			}},
		},
		{
			name: "scored true uses YAML delta",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.Office.Scored = true
					f.Office.Delta = 2
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "Company in Berlin"},
			verdict: &enrichschema.Verdict{Facts: enrichschema.ModelReply{
				Office: enrichschema.Office{
					Countries: []string{"DE"},
					Quote:     quote("Berlin"),
				},
				HiringScope:       enrichschema.HiringScope{Value: "unknown"},
				WorkAuthorization: enrichschema.WorkAuthorization{Value: "unknown"},
				Timezone:          enrichschema.TimezoneConstraint{Value: "unknown"},
				Compensation:      enrichschema.Compensation{Unknown: true},
				WorkMode:          enrichschema.QuotedValue{Value: "unknown"},
				EmploymentLoad:    enrichschema.QuotedValue{Value: "unknown"},
				Engagement:        enrichschema.QuotedValue{Value: "unknown"},
			}},
			want: []schema.Signal{{
				Code:   schema.SignalLLM,
				Rule:   "office",
				Phrase: "Berlin",
				Value:  "DE",
				Delta:  2,
			}},
		},
		{
			name: "mapped phrase on allow-side office emits CONFLICT",
			profile: profileschema.Profile{
				GateHard: []string{"must be based in the us"},
				Facts:    baseFacts,
				Buckets:  review,
			},
			job: jobdata.Job{Description: "Must be based in the US. HQ in Berlin."},
			verdict: &enrichschema.Verdict{Facts: enrichschema.ModelReply{
				Office: enrichschema.Office{
					Countries: []string{"DE"},
					Quote:     quote("Berlin"),
				},
				HiringScope:       enrichschema.HiringScope{Value: "unknown"},
				WorkAuthorization: enrichschema.WorkAuthorization{Value: "unknown"},
				Timezone:          enrichschema.TimezoneConstraint{Value: "unknown"},
				Compensation:      enrichschema.Compensation{Unknown: true},
				WorkMode:          enrichschema.QuotedValue{Value: "unknown"},
				EmploymentLoad:    enrichschema.QuotedValue{Value: "unknown"},
				Engagement:        enrichschema.QuotedValue{Value: "unknown"},
			}},
			want: []schema.Signal{
				{
					Code:        schema.SignalGateHard,
					Rule:        "gate_hard",
					Phrase:      "must be based in the us",
					Occurrences: 1,
				},
				{
					Code:   schema.SignalLLM,
					Rule:   "office",
					Phrase: "Berlin",
					Value:  "DE",
					Delta:  0,
				},
				{
					Code:   schema.SignalConflict,
					Rule:   "office",
					Phrase: "Berlin",
					Value:  "DE",
					Delta:  0,
				},
			},
		},
		{
			name: "unmapped phrase does not emit CONFLICT",
			profile: profileschema.Profile{
				GateHard: []string{"security clearance"},
				Facts:    baseFacts,
				Buckets:  review,
			},
			job: jobdata.Job{Description: "Security clearance required. HQ in Berlin."},
			verdict: &enrichschema.Verdict{Facts: enrichschema.ModelReply{
				Office: enrichschema.Office{
					Countries: []string{"DE"},
					Quote:     quote("Berlin"),
				},
				HiringScope:       enrichschema.HiringScope{Value: "unknown"},
				WorkAuthorization: enrichschema.WorkAuthorization{Value: "unknown"},
				Timezone:          enrichschema.TimezoneConstraint{Value: "unknown"},
				Compensation:      enrichschema.Compensation{Unknown: true},
				WorkMode:          enrichschema.QuotedValue{Value: "unknown"},
				EmploymentLoad:    enrichschema.QuotedValue{Value: "unknown"},
				Engagement:        enrichschema.QuotedValue{Value: "unknown"},
			}},
			want: []schema.Signal{
				{
					Code:        schema.SignalGateHard,
					Rule:        "gate_hard",
					Phrase:      "security clearance",
					Occurrences: 1,
				},
				{
					Code:   schema.SignalLLM,
					Rule:   "office",
					Phrase: "Berlin",
					Value:  "DE",
					Delta:  0,
				},
			},
		},
		{
			name: "cut-side office does not emit CONFLICT",
			profile: profileschema.Profile{
				GateHard: []string{"must be based in the us"},
				Facts:    baseFacts,
				Buckets:  review,
			},
			job: jobdata.Job{Description: "Must be based in the US. Bangalore office."},
			verdict: &enrichschema.Verdict{Facts: enrichschema.ModelReply{
				Office: enrichschema.Office{
					Countries: []string{"IN"},
					Quote:     quote("Bangalore"),
				},
				HiringScope:       enrichschema.HiringScope{Value: "unknown"},
				WorkAuthorization: enrichschema.WorkAuthorization{Value: "unknown"},
				Timezone:          enrichschema.TimezoneConstraint{Value: "unknown"},
				Compensation:      enrichschema.Compensation{Unknown: true},
				WorkMode:          enrichschema.QuotedValue{Value: "unknown"},
				EmploymentLoad:    enrichschema.QuotedValue{Value: "unknown"},
				Engagement:        enrichschema.QuotedValue{Value: "unknown"},
			}},
			want: []schema.Signal{
				{
					Code:        schema.SignalGateHard,
					Rule:        "gate_hard",
					Phrase:      "must be based in the us",
					Occurrences: 1,
				},
				{
					Code:   schema.SignalLLM,
					Rule:   "office",
					Phrase: "Bangalore",
					Value:  "IN",
					Delta:  0,
				},
			},
		},
		{
			name: "employment_load emits LLM and does not gate",
			profile: profileschema.Profile{
				Facts:   baseFacts,
				Buckets: review,
			},
			job: jobdata.Job{Description: "Full-time role"},
			verdict: &enrichschema.Verdict{Facts: enrichschema.ModelReply{
				Office:            enrichschema.Office{Unknown: true},
				HiringScope:       enrichschema.HiringScope{Value: "unknown"},
				WorkAuthorization: enrichschema.WorkAuthorization{Value: "unknown"},
				Timezone:          enrichschema.TimezoneConstraint{Value: "unknown"},
				Compensation:      enrichschema.Compensation{Unknown: true},
				WorkMode:          enrichschema.QuotedValue{Value: "unknown"},
				EmploymentLoad:    enrichschema.QuotedValue{Value: "full_time", Quote: quote("Full-time")},
				Engagement:        enrichschema.QuotedValue{Value: "unknown"},
			}},
			want: []schema.Signal{{
				Code:   schema.SignalLLM,
				Rule:   "employment_load",
				Phrase: "Full-time",
				Value:  "full_time",
				Delta:  0,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := evaluateAt(tt.profile, tt.job, time.Now().UTC(), tt.verdict)
			if !reflect.DeepEqual(got.Signals, tt.want) {
				t.Fatalf("signals got %+v\nwant %+v", got.Signals, tt.want)
			}
			if tt.name == "employment_load emits LLM and does not gate" && got.Bucket == schema.BucketRejected {
				t.Fatalf("employment_load must not gate, bucket=%s", got.Bucket)
			}
			if strings.Contains(tt.name, "CONFLICT") || strings.Contains(tt.name, "mapped phrase") {
				if got.Bucket != schema.BucketRejected {
					t.Fatalf("phrase gate must still reject, bucket=%s", got.Bucket)
				}
			}
		})
	}
}

func TestFactCuts(t *testing.T) {
	t.Parallel()

	quote := func(s string) *string { return &s }
	review := profileschema.Buckets{Top: 1}
	unknownRest := enrichschema.ModelReply{
		Office:            enrichschema.Office{Unknown: true},
		HiringScope:       enrichschema.HiringScope{Value: "unknown"},
		WorkAuthorization: enrichschema.WorkAuthorization{Value: "unknown"},
		Timezone:          enrichschema.TimezoneConstraint{Value: "unknown"},
		Compensation:      enrichschema.Compensation{Unknown: true},
		WorkMode:          enrichschema.QuotedValue{Value: "unknown"},
		EmploymentLoad:    enrichschema.QuotedValue{Value: "unknown"},
		Engagement:        enrichschema.QuotedValue{Value: "unknown"},
	}
	baseFacts := profileschema.Facts{
		ResidenceRegions: []string{"EUROPE"},
		Office: profileschema.FactsOffice{
			AllowCountries: []string{"US", "DE", "FR", "RO", "GB"},
			AllowRegions:   []string{"EUROPE", "EMEA", "WORLDWIDE"},
		},
		WorkAuthorization: profileschema.FactsWorkAuthorization{Gates: []string{"required"}},
		Timezone:          profileschema.FactsTimezone{Allow: []string{"CET", "GMT", "EET"}},
		Compensation:      profileschema.FactsCompensation{AllowCurrencies: []string{"EUR", "USD", "GBP"}},
	}

	tests := []struct {
		name       string
		profile    profileschema.Profile
		job        jobdata.Job
		verdict    *enrichschema.Verdict
		want       []schema.Signal
		wantReview bool // fact cuts land in review, not rejected
	}{
		{
			name: "unscored INR emits LLM and ignores cut",
			profile: profileschema.Profile{
				Facts:   baseFacts,
				Buckets: review,
			},
			job: jobdata.Job{Description: "25 LPA"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.Compensation = enrichschema.Compensation{Currency: "INR", Quote: quote("25 LPA")}
				return f
			}()},
			want: []schema.Signal{{
				Code:   schema.SignalLLM,
				Rule:   "compensation",
				Phrase: "25 LPA",
				Value:  "INR",
				Delta:  0,
			}},
		},
		{
			name: "unknown does not cut when scored",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.Compensation.Scored = true
					f.Office.Scored = true
					f.HiringScope.Scored = true
					f.WorkAuthorization.Scored = true
					f.Timezone.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job:     jobdata.Job{Description: "A role"},
			verdict: &enrichschema.Verdict{Facts: unknownRest},
			want:    nil,
		},
		{
			name: "scored INR fact-cut goes to review",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.Compensation.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "25 LPA"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.Compensation = enrichschema.Compensation{Currency: "INR", Quote: quote("25 LPA")}
				return f
			}()},
			want: []schema.Signal{
				{
					Code:   schema.SignalLLM,
					Rule:   "compensation",
					Phrase: "25 LPA",
					Value:  "INR",
					Delta:  0,
				},
				{
					Code:   schema.SignalGateHard,
					Rule:   "facts.compensation",
					Phrase: "25 LPA",
					Value:  "INR",
					Delta:  0,
				},
			},
			wantReview: true,
		},
		{
			name: "scored INR does not invent an IN office cut",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.Compensation.Scored = true
					f.Office.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "25 LPA"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.Compensation = enrichschema.Compensation{Currency: "INR", Quote: quote("25 LPA")}
				return f
			}()},
			want: []schema.Signal{
				{
					Code:   schema.SignalLLM,
					Rule:   "compensation",
					Phrase: "25 LPA",
					Value:  "INR",
					Delta:  0,
				},
				{
					Code:   schema.SignalGateHard,
					Rule:   "facts.compensation",
					Phrase: "25 LPA",
					Value:  "INR",
					Delta:  0,
				},
			},
			wantReview: true,
		},
		{
			name: "scored office US allowed does not gate",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.Office.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "US company"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.Office = enrichschema.Office{Countries: []string{"US"}, Quote: quote("United States")}
				return f
			}()},
			want: []schema.Signal{{
				Code:   schema.SignalLLM,
				Rule:   "office",
				Phrase: "United States",
				Value:  "US",
				Delta:  0,
			}},
		},
		{
			name: "scored office US outside allow goes to review",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.Office.Scored = true
					f.Office.AllowCountries = []string{"DE", "FR", "RO", "GB"}
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "US company"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.Office = enrichschema.Office{Countries: []string{"US"}, Quote: quote("United States")}
				return f
			}()},
			want: []schema.Signal{
				{
					Code:   schema.SignalLLM,
					Rule:   "office",
					Phrase: "United States",
					Value:  "US",
					Delta:  0,
				},
				{
					Code:   schema.SignalGateHard,
					Rule:   "facts.office",
					Phrase: "United States",
					Value:  "US",
					Delta:  0,
				},
			},
			wantReview: true,
		},
		{
			name: "scored restricted US goes to review against residence EUROPE only",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.HiringScope.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "US residents only"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.HiringScope = enrichschema.HiringScope{
					Value:     "restricted",
					Countries: []string{"US"},
					Quote:     quote("US residents only"),
				}
				return f
			}()},
			want: []schema.Signal{
				{
					Code:   schema.SignalLLM,
					Rule:   "hiring_scope",
					Phrase: "US residents only",
					Value:  "US",
					Delta:  0,
				},
				{
					Code:   schema.SignalGateHard,
					Rule:   "facts.hiring_scope",
					Phrase: "US residents only",
					Value:  "US",
					Delta:  0,
				},
			},
			wantReview: true,
		},
		{
			name: "scored restricted US allowed when residence includes US",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.ResidenceRegions = []string{"EUROPE", "US"}
					f.HiringScope.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "US residents only"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.HiringScope = enrichschema.HiringScope{
					Value:     "restricted",
					Countries: []string{"US"},
					Quote:     quote("US residents only"),
				}
				return f
			}()},
			want: []schema.Signal{{
				Code:   schema.SignalLLM,
				Rule:   "hiring_scope",
				Phrase: "US residents only",
				Value:  "US",
				Delta:  0,
			}},
		},
		{
			name: "scored restricted FR does not gate against residence EUROPE",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.HiringScope.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "Candidates in France"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.HiringScope = enrichschema.HiringScope{
					Value:     "restricted",
					Countries: []string{"FR"},
					Quote:     quote("Candidates in France"),
				}
				return f
			}()},
			want: []schema.Signal{{
				Code:   schema.SignalLLM,
				Rule:   "hiring_scope",
				Phrase: "Candidates in France",
				Value:  "FR",
				Delta:  0,
			}},
		},
		{
			name: "scored restricted EMEA does not gate against residence EUROPE",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.HiringScope.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "EMEA only"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.HiringScope = enrichschema.HiringScope{
					Value:   "restricted",
					Regions: []string{"EMEA"},
					Quote:   quote("EMEA only"),
				}
				return f
			}()},
			want: []schema.Signal{{
				Code:   schema.SignalLLM,
				Rule:   "hiring_scope",
				Phrase: "EMEA only",
				Value:  "EMEA",
				Delta:  0,
			}},
		},
		{
			name: "scored restricted LATAM goes to review against residence EUROPE",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.HiringScope.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "LATAM only"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.HiringScope = enrichschema.HiringScope{
					Value:   "restricted",
					Regions: []string{"LATAM"},
					Quote:   quote("LATAM only"),
				}
				return f
			}()},
			want: []schema.Signal{
				{
					Code:   schema.SignalLLM,
					Rule:   "hiring_scope",
					Phrase: "LATAM only",
					Value:  "LATAM",
					Delta:  0,
				},
				{
					Code:   schema.SignalGateHard,
					Rule:   "facts.hiring_scope",
					Phrase: "LATAM only",
					Value:  "LATAM",
					Delta:  0,
				},
			},
			wantReview: true,
		},
		{
			name: "fact cut demotes would-be top into review",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.HiringScope.Scored = true
					return f
				}(),
				Buckets: profileschema.Buckets{Top: 0},
			},
			job: jobdata.Job{Description: "LATAM only"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.HiringScope = enrichschema.HiringScope{
					Value:   "restricted",
					Regions: []string{"LATAM"},
					Quote:   quote("LATAM only"),
				}
				return f
			}()},
			want: []schema.Signal{
				{
					Code:   schema.SignalLLM,
					Rule:   "hiring_scope",
					Phrase: "LATAM only",
					Value:  "LATAM",
					Delta:  0,
				},
				{
					Code:   schema.SignalGateHard,
					Rule:   "facts.hiring_scope",
					Phrase: "LATAM only",
					Value:  "LATAM",
					Delta:  0,
				},
			},
			wantReview: true,
		},
		{
			name: "scored work_authorization required goes to review",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.WorkAuthorization.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "Authorized to work in the US"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.WorkAuthorization = enrichschema.WorkAuthorization{
					Value: "required",
					Quote: quote("Authorized to work in the US"),
				}
				return f
			}()},
			want: []schema.Signal{
				{
					Code:   schema.SignalLLM,
					Rule:   "work_authorization",
					Phrase: "Authorized to work in the US",
					Value:  "required",
					Delta:  0,
				},
				{
					Code:   schema.SignalGateHard,
					Rule:   "facts.work_authorization",
					Phrase: "Authorized to work in the US",
					Value:  "required",
					Delta:  0,
				},
			},
			wantReview: true,
		},
		{
			name: "scored timezone restricted with no allow zone goes to review",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.Timezone.Scored = true
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "US Eastern hours"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.Timezone = enrichschema.TimezoneConstraint{
					Value: "restricted",
					Zones: []string{"US_EASTERN"},
					Quote: quote("US Eastern hours"),
				}
				return f
			}()},
			want: []schema.Signal{
				{
					Code:   schema.SignalLLM,
					Rule:   "timezone",
					Phrase: "US Eastern hours",
					Value:  "US_EASTERN",
					Delta:  0,
				},
				{
					Code:   schema.SignalGateHard,
					Rule:   "facts.timezone",
					Phrase: "US Eastern hours",
					Value:  "US_EASTERN",
					Delta:  0,
				},
			},
			wantReview: true,
		},
		{
			name: "scored employment_load does not gate",
			profile: profileschema.Profile{
				Facts: func() profileschema.Facts {
					f := baseFacts
					f.EmploymentLoad.Scored = true
					f.EmploymentLoad.Delta = 1
					return f
				}(),
				Buckets: review,
			},
			job: jobdata.Job{Description: "Full-time role"},
			verdict: &enrichschema.Verdict{Facts: func() enrichschema.ModelReply {
				f := unknownRest
				f.EmploymentLoad = enrichschema.QuotedValue{Value: "full_time", Quote: quote("Full-time")}
				return f
			}()},
			want: []schema.Signal{{
				Code:   schema.SignalLLM,
				Rule:   "employment_load",
				Phrase: "Full-time",
				Value:  "full_time",
				Delta:  1,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := evaluateAt(tt.profile, tt.job, time.Now().UTC(), tt.verdict)
			if !reflect.DeepEqual(got.Signals, tt.want) {
				t.Fatalf("signals got %+v\nwant %+v", got.Signals, tt.want)
			}
			if got.Bucket == schema.BucketRejected {
				t.Fatalf("fact cuts must not reject, bucket=%s", got.Bucket)
			}
			if tt.wantReview && got.Bucket != schema.BucketReview {
				t.Fatalf("bucket=%s want REVIEW", got.Bucket)
			}
		})
	}
}
