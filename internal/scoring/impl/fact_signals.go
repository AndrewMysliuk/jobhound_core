package impl

import (
	"strings"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	enrichschema "github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// factSignals emits LLM, CONFLICT, and scored GATE_HARD cuts from a ready verdict.
// scored:false still emits LLM with delta 0 and skips cuts. unknown emits nothing for that field.
// employment_load and engagement never cut. Fact GATE_HARD rules (facts.*) go to review, not rejected.
func factSignals(profile profileschema.Profile, job jobdata.Job, verdict enrichschema.Verdict) []schema.Signal {
	facts := profile.Facts
	var signals []schema.Signal

	signals = append(signals, officeFactSignals(facts.Office, verdict.Facts.Office)...)
	signals = append(signals, officeCutSignals(facts.Office, verdict.Facts.Office, verdict.Facts.WorkMode, verdict.Facts.HiringScope)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameOffice, officeOnAllowSide(verdict.Facts.Office, facts.Office), job, quoteOf(verdict.Facts.Office.Quote), officePrimaryValue(verdict.Facts.Office))...)

	signals = append(signals, hiringScopeFactSignals(facts.HiringScope, verdict.Facts.HiringScope)...)
	signals = append(signals, hiringScopeCutSignals(facts.HiringScope, facts.ResidenceRegions, verdict.Facts.HiringScope)...)
	signals = append(signals, hiringScopeNarrowPenalty(facts.HiringScope, facts.ResidenceRegions, facts.ResidenceCountries, verdict.Facts.HiringScope)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameHiringScope, hiringScopeOnAllowSide(verdict.Facts.HiringScope, facts.ResidenceRegions), job, quoteOf(verdict.Facts.HiringScope.Quote), hiringScopePrimaryValue(verdict.Facts.HiringScope))...)

	signals = append(signals, workAuthFactSignals(facts.WorkAuthorization, verdict.Facts.WorkAuthorization)...)
	signals = append(signals, workAuthCutSignals(facts.WorkAuthorization, verdict.Facts.WorkAuthorization, facts.ResidenceCountries, facts.ResidenceRegions)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameWorkAuthorization, workAuthOnAllowSide(verdict.Facts.WorkAuthorization, facts.WorkAuthorization), job, quoteOf(verdict.Facts.WorkAuthorization.Quote), strings.TrimSpace(verdict.Facts.WorkAuthorization.Value))...)

	signals = append(signals, timezoneFactSignals(facts.Timezone, verdict.Facts.Timezone)...)
	signals = append(signals, timezoneCutSignals(facts.Timezone, verdict.Facts.Timezone)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameTimezone, timezoneOnAllowSide(verdict.Facts.Timezone, facts.Timezone), job, quoteOf(verdict.Facts.Timezone.Quote), timezonePrimaryValue(verdict.Facts.Timezone))...)

	signals = append(signals, compensationFactSignals(facts.Compensation, verdict.Facts.Compensation)...)
	signals = append(signals, compensationCutSignals(facts.Compensation, verdict.Facts.Compensation)...)
	// compensation has no mapped phrases in this slice; conflictIf is a no-op.

	signals = append(signals, workModeFactSignals(facts.WorkMode, verdict.Facts.WorkMode)...)
	signals = append(signals, workModeCutSignals(facts.WorkMode, verdict.Facts.WorkMode)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameWorkMode, workModeOnAllowSide(verdict.Facts.WorkMode, facts.WorkMode), job, quoteOf(verdict.Facts.WorkMode.Quote), strings.TrimSpace(verdict.Facts.WorkMode.Value))...)

	signals = append(signals, quotedFactSignals(enrichschema.FieldNameEmploymentLoad.String(), facts.EmploymentLoad, verdict.Facts.EmploymentLoad)...)
	signals = append(signals, quotedFactSignals(enrichschema.FieldNameEngagement.String(), facts.Engagement, verdict.Facts.Engagement)...)

	return signals
}

func officeCutSignals(cfg profileschema.FactsOffice, office enrichschema.Office, workMode enrichschema.QuotedValue, scope enrichschema.HiringScope) []schema.Signal {
	if !cfg.Scored || office.Unknown {
		return nil
	}
	// HQ location is not a presence requirement when the role is fully remote
	// and hiring_scope already allows Europe / worldwide.
	if officeCutSuppressed(workMode, scope) {
		return nil
	}
	allowC := compact(cfg.AllowCountries)
	allowR := compact(cfg.AllowRegions)
	quote := quoteOf(office.Quote)
	var out []schema.Signal
	for _, code := range compact(office.Countries) {
		if missesAllowLists(code, allowC, allowR) {
			out = append(out, factGate(enrichschema.FieldNameOffice, quote, code))
		}
	}
	for _, code := range compact(office.Regions) {
		if missesAllowLists(code, allowC, allowR) {
			out = append(out, factGate(enrichschema.FieldNameOffice, quote, code))
		}
	}
	return out
}

// officeCutSuppressed reports remote + Europe-compatible hiring_scope.
// Unknown scope keeps the office cut (HQ is the only location signal).
func officeCutSuppressed(workMode enrichschema.QuotedValue, scope enrichschema.HiringScope) bool {
	if !enrichschema.WorkModeRemote.Equals(strings.TrimSpace(workMode.Value)) {
		return false
	}
	return hiringScopeEuropeCompatible(scope)
}

// hiringScopeEuropeCompatible is worldwide, or restricted that intersects EUROPE.
func hiringScopeEuropeCompatible(scope enrichschema.HiringScope) bool {
	if isUnknownValue(scope.Value) {
		return false
	}
	if enrichschema.ScopeWorldwide.Equals(scope.Value) {
		return true
	}
	if !enrichschema.ScopeRestricted.Equals(scope.Value) {
		return false
	}
	return residenceIntersects([]string{"EUROPE"}, scope.Countries, scope.Regions)
}

func hiringScopeCutSignals(cfg profileschema.FactsHiringScope, residence []string, scope enrichschema.HiringScope) []schema.Signal {
	if !cfg.Scored || isUnknownValue(scope.Value) {
		return nil
	}
	if !enrichschema.ScopeRestricted.Equals(scope.Value) {
		return nil
	}
	if residenceIntersects(residence, scope.Countries, scope.Regions) {
		return nil
	}
	return []schema.Signal{factGate(enrichschema.FieldNameHiringScope, quoteOf(scope.Quote), hiringScopePrimaryValue(scope))}
}

// maxNarrowScopeCountries is the largest restricted country list that still
// counts as "narrow" for residence_countries soft penalty. Broader lists and
// EUROPE/EMEA/WORLDWIDE regions are left alone.
const maxNarrowScopeCountries = 2

// hiringScopeNarrowPenalty soft-hits when restricted scope lists 1–2 countries
// that omit residence_countries and has no broad region. Skips when the hard
// residence_regions cut already applies. Does not reject.
func hiringScopeNarrowPenalty(cfg profileschema.FactsHiringScope, residenceRegions, residenceCountries []string, scope enrichschema.HiringScope) []schema.Signal {
	if !cfg.Scored || cfg.NarrowPenalty == 0 || isUnknownValue(scope.Value) {
		return nil
	}
	if !enrichschema.ScopeRestricted.Equals(scope.Value) {
		return nil
	}
	if !residenceIntersects(residenceRegions, scope.Countries, scope.Regions) {
		return nil
	}
	residence := compact(residenceCountries)
	if len(residence) == 0 {
		return nil
	}
	if hiringScopeBroadRegion(scope.Regions) {
		return nil
	}
	countries := compact(scope.Countries)
	if len(countries) == 0 || len(countries) > maxNarrowScopeCountries {
		return nil
	}
	for _, code := range residence {
		if containsFold(countries, code) {
			return nil
		}
	}
	return []schema.Signal{{
		Code:   schema.SignalPenalty,
		Rule:   "facts.hiring_scope.narrow",
		Phrase: quoteOf(scope.Quote),
		Value:  hiringScopePrimaryValue(scope),
		Delta:  cfg.NarrowPenalty,
	}}
}

func hiringScopeBroadRegion(regions []string) bool {
	for _, region := range compact(regions) {
		switch strings.ToUpper(region) {
		case "EUROPE", "EMEA", "WORLDWIDE":
			return true
		}
	}
	return false
}

func workAuthCutSignals(
	cfg profileschema.FactsWorkAuthorization,
	auth enrichschema.WorkAuthorization,
	residenceCountries, residenceRegions []string,
) []schema.Signal {
	if !cfg.Scored || isUnknownValue(auth.Value) {
		return nil
	}
	value := strings.TrimSpace(auth.Value)
	gated := false
	for _, gate := range cfg.Gates {
		if strings.EqualFold(strings.TrimSpace(gate), value) {
			gated = true
			break
		}
	}
	if !gated {
		return nil
	}
	countries := compact(auth.Countries)
	if len(countries) == 0 {
		// Owner decision: a gate that cannot name a country does not reject;
		// the vacancy still competes on score and may become TOP.
		return nil
	}
	allowC := compact(residenceCountries)
	allowR := compact(residenceRegions)
	for _, code := range countries {
		if !missesAllowLists(code, allowC, allowR) {
			return nil
		}
	}
	return []schema.Signal{factGate(enrichschema.FieldNameWorkAuthorization, quoteOf(auth.Quote), value)}
}

func timezoneCutSignals(cfg profileschema.FactsTimezone, tz enrichschema.TimezoneConstraint) []schema.Signal {
	if !cfg.Scored || isUnknownValue(tz.Value) {
		return nil
	}
	if timezoneOpen(tz.Value) || !enrichschema.ScopeRestricted.Equals(tz.Value) {
		return nil
	}
	allow := compact(cfg.Allow)
	for _, zone := range compact(tz.Zones) {
		if containsFold(allow, zone) {
			return nil
		}
	}
	return []schema.Signal{factGate(enrichschema.FieldNameTimezone, quoteOf(tz.Quote), timezonePrimaryValue(tz))}
}

func compensationCutSignals(cfg profileschema.FactsCompensation, pay enrichschema.Compensation) []schema.Signal {
	if !cfg.Scored || pay.Unknown {
		return nil
	}
	currency := strings.TrimSpace(pay.Currency)
	if currency == "" {
		return nil
	}
	if containsFold(compact(cfg.AllowCurrencies), currency) {
		return nil
	}
	return []schema.Signal{factGate(enrichschema.FieldNameCompensation, quoteOf(pay.Quote), currency)}
}

func factGate(field enrichschema.FieldName, phrase, value string) schema.Signal {
	return schema.Signal{
		Code:   schema.SignalGateHard,
		Rule:   "facts." + field.String(),
		Phrase: phrase,
		Value:  value,
		Delta:  0,
	}
}

func conflictIf(field enrichschema.FieldName, onAllowSide bool, job jobdata.Job, quote, value string) []schema.Signal {
	if !onAllowSide || value == "" || isUnknownValue(value) {
		return nil
	}
	if !mappedPhraseFired(field, job) {
		return nil
	}
	return []schema.Signal{{
		Code:   schema.SignalConflict,
		Rule:   field.String(),
		Phrase: quote,
		Value:  value,
		Delta:  0,
	}}
}

func mappedPhraseFired(field enrichschema.FieldName, job jobdata.Job) bool {
	for _, m := range factPhraseMaps {
		if m.Field != field {
			continue
		}
		for _, phrase := range m.Phrases {
			if phraseHit(job, phrase) {
				return true
			}
		}
	}
	return false
}

func phraseHit(job jobdata.Job, phrase string) bool {
	return activeHits(job.Title, phrase, true) > 0 ||
		activeHits(job.Description, phrase, true) > 0 ||
		activeHits(job.Location.Raw, phrase, true) > 0 ||
		activeHits(job.Location.Timezone, phrase, true) > 0
}

func officeFactSignals(cfg profileschema.FactsOffice, office enrichschema.Office) []schema.Signal {
	if office.Unknown {
		return nil
	}
	countries := compact(office.Countries)
	regions := compact(office.Regions)
	if len(countries) == 0 && len(regions) == 0 {
		return nil
	}
	delta := factDelta(cfg.Scored, cfg.Delta)
	quote := quoteOf(office.Quote)
	out := make([]schema.Signal, 0, len(countries)+len(regions))
	for _, code := range countries {
		out = append(out, llmSignal(enrichschema.FieldNameOffice.String(), quote, code, delta))
	}
	for _, code := range regions {
		out = append(out, llmSignal(enrichschema.FieldNameOffice.String(), quote, code, delta))
	}
	return out
}

func hiringScopeFactSignals(cfg profileschema.FactsHiringScope, scope enrichschema.HiringScope) []schema.Signal {
	if isUnknownValue(scope.Value) {
		return nil
	}
	quote := quoteOf(scope.Quote)
	bonus := 0
	if cfg.Scored && cfg.Delta != 0 && hiringScopeBonusEligible(scope) {
		bonus = cfg.Delta
	}
	if enrichschema.ScopeWorldwide.Equals(scope.Value) {
		return []schema.Signal{llmSignal(enrichschema.FieldNameHiringScope.String(), quote, enrichschema.ScopeWorldwide.String(), bonus)}
	}
	countries := compact(scope.Countries)
	regions := compact(scope.Regions)
	if len(countries) == 0 && len(regions) == 0 {
		return []schema.Signal{llmSignal(enrichschema.FieldNameHiringScope.String(), quote, strings.TrimSpace(scope.Value), 0)}
	}
	out := make([]schema.Signal, 0, len(countries)+len(regions))
	for _, code := range countries {
		out = append(out, llmSignal(enrichschema.FieldNameHiringScope.String(), quote, code, 0))
	}
	for _, code := range regions {
		d := 0
		if bonus != 0 && hiringScopeBonusRegion(code) {
			d = bonus
			bonus = 0
		}
		out = append(out, llmSignal(enrichschema.FieldNameHiringScope.String(), quote, code, d))
	}
	// Restricted to countries only (no broad region): no scope bonus.
	return out
}

// hiringScopeBonusEligible is worldwide, or restricted with EUROPE/EMEA/WORLDWIDE.
func hiringScopeBonusEligible(scope enrichschema.HiringScope) bool {
	if enrichschema.ScopeWorldwide.Equals(scope.Value) {
		return true
	}
	if !enrichschema.ScopeRestricted.Equals(scope.Value) {
		return false
	}
	for _, region := range compact(scope.Regions) {
		if hiringScopeBonusRegion(region) {
			return true
		}
	}
	return false
}

func hiringScopeBonusRegion(region string) bool {
	switch strings.ToUpper(strings.TrimSpace(region)) {
	case "EUROPE", "EMEA", "WORLDWIDE":
		return true
	default:
		return false
	}
}

func workAuthFactSignals(cfg profileschema.FactsWorkAuthorization, auth enrichschema.WorkAuthorization) []schema.Signal {
	if isUnknownValue(auth.Value) {
		return nil
	}
	return []schema.Signal{llmSignal(
		enrichschema.FieldNameWorkAuthorization.String(),
		quoteOf(auth.Quote),
		strings.TrimSpace(auth.Value),
		factDelta(cfg.Scored, cfg.Delta),
	)}
}

func timezoneFactSignals(cfg profileschema.FactsTimezone, tz enrichschema.TimezoneConstraint) []schema.Signal {
	if isUnknownValue(tz.Value) {
		return nil
	}
	delta := factDelta(cfg.Scored, cfg.Delta)
	quote := quoteOf(tz.Quote)
	if timezoneOpen(tz.Value) {
		return []schema.Signal{llmSignal(enrichschema.FieldNameTimezone.String(), quote, strings.TrimSpace(tz.Value), delta)}
	}
	zones := compact(tz.Zones)
	if len(zones) == 0 {
		return []schema.Signal{llmSignal(enrichschema.FieldNameTimezone.String(), quote, strings.TrimSpace(tz.Value), delta)}
	}
	out := make([]schema.Signal, 0, len(zones))
	for _, zone := range zones {
		out = append(out, llmSignal(enrichschema.FieldNameTimezone.String(), quote, zone, delta))
	}
	return out
}

func compensationFactSignals(cfg profileschema.FactsCompensation, pay enrichschema.Compensation) []schema.Signal {
	if pay.Unknown || strings.TrimSpace(pay.Currency) == "" {
		return nil
	}
	return []schema.Signal{llmSignal(
		enrichschema.FieldNameCompensation.String(),
		quoteOf(pay.Quote),
		strings.TrimSpace(pay.Currency),
		factDelta(cfg.Scored, cfg.Delta),
	)}
}

func quotedFactSignals(rule string, cfg profileschema.FactField, value enrichschema.QuotedValue) []schema.Signal {
	if isUnknownValue(value.Value) {
		return nil
	}
	return []schema.Signal{llmSignal(rule, quoteOf(value.Quote), strings.TrimSpace(value.Value), factDelta(cfg.Scored, cfg.Delta))}
}

func llmSignal(rule, phrase, value string, delta int) schema.Signal {
	return schema.Signal{
		Code:   schema.SignalLLM,
		Rule:   rule,
		Phrase: phrase,
		Value:  value,
		Delta:  delta,
	}
}

func factDelta(scored bool, delta int) int {
	if !scored {
		return 0
	}
	return delta
}

func quoteOf(q *string) string {
	if q == nil {
		return ""
	}
	return *q
}

func isUnknownValue(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "unknown") || strings.TrimSpace(v) == ""
}

func timezoneOpen(value string) bool {
	v := strings.TrimSpace(value)
	return strings.EqualFold(v, "open") || enrichschema.ScopeWorldwide.Equals(v)
}

func officeOnAllowSide(office enrichschema.Office, cfg profileschema.FactsOffice) bool {
	if office.Unknown {
		return false
	}
	countries := compact(office.Countries)
	regions := compact(office.Regions)
	if len(countries) == 0 && len(regions) == 0 {
		return false
	}
	allowC := compact(cfg.AllowCountries)
	allowR := compact(cfg.AllowRegions)
	if len(allowC) == 0 && len(allowR) == 0 {
		return true
	}
	for _, code := range countries {
		if missesAllowLists(code, allowC, allowR) {
			return false
		}
	}
	for _, code := range regions {
		if missesAllowLists(code, allowC, allowR) {
			return false
		}
	}
	return true
}

func hiringScopeOnAllowSide(scope enrichschema.HiringScope, residence []string) bool {
	if isUnknownValue(scope.Value) {
		return false
	}
	if enrichschema.ScopeWorldwide.Equals(scope.Value) {
		return true
	}
	if !enrichschema.ScopeRestricted.Equals(scope.Value) {
		return false
	}
	return residenceIntersects(residence, scope.Countries, scope.Regions)
}

func workAuthOnAllowSide(auth enrichschema.WorkAuthorization, cfg profileschema.FactsWorkAuthorization) bool {
	if isUnknownValue(auth.Value) {
		return false
	}
	for _, gate := range cfg.Gates {
		if strings.EqualFold(strings.TrimSpace(gate), strings.TrimSpace(auth.Value)) {
			return false
		}
	}
	return true
}

func timezoneOnAllowSide(tz enrichschema.TimezoneConstraint, cfg profileschema.FactsTimezone) bool {
	if isUnknownValue(tz.Value) {
		return false
	}
	if timezoneOpen(tz.Value) {
		return true
	}
	if !enrichschema.ScopeRestricted.Equals(tz.Value) {
		return false
	}
	allow := compact(cfg.Allow)
	for _, zone := range compact(tz.Zones) {
		if containsFold(allow, zone) {
			return true
		}
	}
	return false
}

func workModeFactSignals(cfg profileschema.FactsWorkMode, mode enrichschema.QuotedValue) []schema.Signal {
	if isUnknownValue(mode.Value) {
		return nil
	}
	return []schema.Signal{llmSignal(
		enrichschema.FieldNameWorkMode.String(),
		quoteOf(mode.Quote),
		strings.TrimSpace(mode.Value),
		factDelta(cfg.Scored, cfg.Delta),
	)}
}

func workModeCutSignals(cfg profileschema.FactsWorkMode, mode enrichschema.QuotedValue) []schema.Signal {
	if !cfg.Scored || isUnknownValue(mode.Value) {
		return nil
	}
	value := strings.TrimSpace(mode.Value)
	for _, gate := range cfg.Gates {
		if strings.EqualFold(strings.TrimSpace(gate), value) {
			return []schema.Signal{factGate(enrichschema.FieldNameWorkMode, quoteOf(mode.Quote), value)}
		}
	}
	return nil
}

func workModeOnAllowSide(mode enrichschema.QuotedValue, cfg profileschema.FactsWorkMode) bool {
	if isUnknownValue(mode.Value) {
		return false
	}
	value := strings.TrimSpace(mode.Value)
	for _, gate := range cfg.Gates {
		if strings.EqualFold(strings.TrimSpace(gate), value) {
			return false
		}
	}
	return enrichschema.WorkModeRemote.Equals(value)
}

// residenceIntersects reports whether a restricted hiring scope meets residence_regions.
// EUROPE residence: a country in the scorer Europe set, or region EUROPE / WORLDWIDE / EMEA.
func residenceIntersects(residence, countries, regions []string) bool {
	for _, res := range compact(residence) {
		switch strings.ToUpper(res) {
		case "WORLDWIDE":
			return true
		case "EUROPE":
			for _, code := range compact(countries) {
				if europeCountry(code) {
					return true
				}
			}
			for _, region := range compact(regions) {
				switch strings.ToUpper(region) {
				case "EUROPE", "WORLDWIDE", "EMEA":
					return true
				}
			}
		case "EMEA":
			for _, code := range compact(countries) {
				if europeCountry(code) {
					return true
				}
			}
			for _, region := range compact(regions) {
				switch strings.ToUpper(region) {
				case "EUROPE", "WORLDWIDE", "EMEA":
					return true
				}
			}
		default:
			if containsFold(countries, res) || containsFold(regions, res) {
				return true
			}
		}
	}
	return false
}

func officePrimaryValue(office enrichschema.Office) string {
	if countries := compact(office.Countries); len(countries) > 0 {
		return countries[0]
	}
	if regions := compact(office.Regions); len(regions) > 0 {
		return regions[0]
	}
	return ""
}

func hiringScopePrimaryValue(scope enrichschema.HiringScope) string {
	if enrichschema.ScopeWorldwide.Equals(scope.Value) {
		return enrichschema.ScopeWorldwide.String()
	}
	if countries := compact(scope.Countries); len(countries) > 0 {
		return countries[0]
	}
	if regions := compact(scope.Regions); len(regions) > 0 {
		return regions[0]
	}
	return strings.TrimSpace(scope.Value)
}

func timezonePrimaryValue(tz enrichschema.TimezoneConstraint) string {
	if timezoneOpen(tz.Value) {
		return strings.TrimSpace(tz.Value)
	}
	if zones := compact(tz.Zones); len(zones) > 0 {
		return zones[0]
	}
	return strings.TrimSpace(tz.Value)
}
