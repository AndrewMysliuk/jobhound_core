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
	signals = append(signals, officeCutSignals(facts.Office, verdict.Facts.Office)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameOffice, officeOnAllowSide(verdict.Facts.Office, facts.Office), job, quoteOf(verdict.Facts.Office.Quote), officePrimaryValue(verdict.Facts.Office))...)

	signals = append(signals, hiringScopeFactSignals(facts.HiringScope, verdict.Facts.HiringScope)...)
	signals = append(signals, hiringScopeCutSignals(facts.HiringScope, facts.ResidenceRegions, verdict.Facts.HiringScope)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameHiringScope, hiringScopeOnAllowSide(verdict.Facts.HiringScope, facts.ResidenceRegions), job, quoteOf(verdict.Facts.HiringScope.Quote), hiringScopePrimaryValue(verdict.Facts.HiringScope))...)

	signals = append(signals, workAuthFactSignals(facts.WorkAuthorization, verdict.Facts.WorkAuthorization)...)
	signals = append(signals, workAuthCutSignals(facts.WorkAuthorization, verdict.Facts.WorkAuthorization)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameWorkAuthorization, workAuthOnAllowSide(verdict.Facts.WorkAuthorization, facts.WorkAuthorization), job, quoteOf(verdict.Facts.WorkAuthorization.Quote), strings.TrimSpace(verdict.Facts.WorkAuthorization.Value))...)

	signals = append(signals, timezoneFactSignals(facts.Timezone, verdict.Facts.Timezone)...)
	signals = append(signals, timezoneCutSignals(facts.Timezone, verdict.Facts.Timezone)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameTimezone, timezoneOnAllowSide(verdict.Facts.Timezone, facts.Timezone), job, quoteOf(verdict.Facts.Timezone.Quote), timezonePrimaryValue(verdict.Facts.Timezone))...)

	signals = append(signals, compensationFactSignals(facts.Compensation, verdict.Facts.Compensation)...)
	signals = append(signals, compensationCutSignals(facts.Compensation, verdict.Facts.Compensation)...)
	// compensation has no mapped phrases in this slice; conflictIf is a no-op.

	signals = append(signals, quotedFactSignals(enrichschema.FieldNameWorkMode.String(), facts.WorkMode, verdict.Facts.WorkMode)...)
	signals = append(signals, conflictIf(enrichschema.FieldNameWorkMode, workModeOnAllowSide(verdict.Facts.WorkMode), job, quoteOf(verdict.Facts.WorkMode.Quote), strings.TrimSpace(verdict.Facts.WorkMode.Value))...)

	signals = append(signals, quotedFactSignals(enrichschema.FieldNameEmploymentLoad.String(), facts.EmploymentLoad, verdict.Facts.EmploymentLoad)...)
	signals = append(signals, quotedFactSignals(enrichschema.FieldNameEngagement.String(), facts.Engagement, verdict.Facts.Engagement)...)

	return signals
}

func officeCutSignals(cfg profileschema.FactsOffice, office enrichschema.Office) []schema.Signal {
	if !cfg.Scored || office.Unknown {
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

func hiringScopeCutSignals(cfg profileschema.FactField, residence []string, scope enrichschema.HiringScope) []schema.Signal {
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

func workAuthCutSignals(cfg profileschema.FactsWorkAuthorization, auth enrichschema.WorkAuthorization) []schema.Signal {
	if !cfg.Scored || isUnknownValue(auth.Value) {
		return nil
	}
	value := strings.TrimSpace(auth.Value)
	for _, gate := range cfg.Gates {
		if strings.EqualFold(strings.TrimSpace(gate), value) {
			return []schema.Signal{factGate(enrichschema.FieldNameWorkAuthorization, quoteOf(auth.Quote), value)}
		}
	}
	return nil
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

func hiringScopeFactSignals(cfg profileschema.FactField, scope enrichschema.HiringScope) []schema.Signal {
	if isUnknownValue(scope.Value) {
		return nil
	}
	delta := factDelta(cfg.Scored, cfg.Delta)
	quote := quoteOf(scope.Quote)
	if enrichschema.ScopeWorldwide.Equals(scope.Value) {
		return []schema.Signal{llmSignal(enrichschema.FieldNameHiringScope.String(), quote, enrichschema.ScopeWorldwide.String(), delta)}
	}
	countries := compact(scope.Countries)
	regions := compact(scope.Regions)
	if len(countries) == 0 && len(regions) == 0 {
		return []schema.Signal{llmSignal(enrichschema.FieldNameHiringScope.String(), quote, strings.TrimSpace(scope.Value), delta)}
	}
	out := make([]schema.Signal, 0, len(countries)+len(regions))
	for _, code := range countries {
		out = append(out, llmSignal(enrichschema.FieldNameHiringScope.String(), quote, code, delta))
	}
	for _, code := range regions {
		out = append(out, llmSignal(enrichschema.FieldNameHiringScope.String(), quote, code, delta))
	}
	return out
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

func workModeOnAllowSide(mode enrichschema.QuotedValue) bool {
	return enrichschema.WorkModeRemote.Equals(strings.TrimSpace(mode.Value))
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
