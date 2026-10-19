package impl

import (
	"strings"

	"golang.org/x/text/currency"
	"golang.org/x/text/language"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
)

const unknownAnswer = "unknown"

// CoerceModelReply normalizes a decoded ModelReply against vacancyMatchText.
// It checks quotes and drops invalid country, currency, region, and zone codes.
// It does not interpret the vacancy (no language, badge, or visa detection).
// A value the model left unknown stays unknown. Currency is never copied into office.
func CoerceModelReply(reply schema.ModelReply, vacancyMatchText string) schema.ModelReply {
	return schema.ModelReply{
		Office:            coerceOffice(reply.Office, vacancyMatchText),
		HiringScope:       coerceHiringScope(reply.HiringScope, vacancyMatchText),
		WorkAuthorization: coerceWorkAuthorization(reply.WorkAuthorization, vacancyMatchText),
		Timezone:          coerceTimezone(reply.Timezone, vacancyMatchText),
		Compensation:      coerceCompensation(reply.Compensation, vacancyMatchText),
		WorkMode:          coerceQuotedValue(reply.WorkMode, vacancyMatchText),
		EmploymentLoad:    coerceQuotedValue(reply.EmploymentLoad, vacancyMatchText),
		Engagement:        coerceQuotedValue(reply.Engagement, vacancyMatchText),
	}
}

func coerceOffice(o schema.Office, vacancyMatchText string) schema.Office {
	if o.Unknown {
		return unknownOffice()
	}
	if !quoteOK(o.Quote, vacancyMatchText) {
		return unknownOffice()
	}
	countries := filterCountries(o.Countries)
	regions := filterRegions(o.Regions)
	if len(countries) == 0 && len(regions) == 0 {
		return unknownOffice()
	}
	return schema.Office{
		Countries: countries,
		Regions:   regions,
		Quote:     o.Quote,
		Unknown:   false,
	}
}

func coerceHiringScope(h schema.HiringScope, vacancyMatchText string) schema.HiringScope {
	if h.Value == unknownAnswer || h.Value == "" {
		return unknownHiringScope()
	}
	if !quoteOK(h.Quote, vacancyMatchText) {
		return unknownHiringScope()
	}
	switch h.Value {
	case schema.ScopeWorldwide.String():
		return schema.HiringScope{
			Value:     schema.ScopeWorldwide.String(),
			Countries: emptyStrings(),
			Regions:   emptyStrings(),
			Quote:     h.Quote,
		}
	case schema.ScopeRestricted.String():
		countries := filterCountries(h.Countries)
		regions := filterRegions(h.Regions)
		if len(countries) == 0 && len(regions) == 0 {
			return unknownHiringScope()
		}
		return schema.HiringScope{
			Value:     schema.ScopeRestricted.String(),
			Countries: countries,
			Regions:   regions,
			Quote:     h.Quote,
		}
	default:
		return unknownHiringScope()
	}
}

func coerceWorkAuthorization(a schema.WorkAuthorization, vacancyMatchText string) schema.WorkAuthorization {
	if a.Value == unknownAnswer || a.Value == "" {
		return unknownWorkAuthorization()
	}
	if !quoteOK(a.Quote, vacancyMatchText) {
		return unknownWorkAuthorization()
	}
	switch a.Value {
	case schema.AuthorizationNone.String():
		return schema.WorkAuthorization{
			Value:     schema.AuthorizationNone.String(),
			Countries: emptyStrings(),
			Quote:     a.Quote,
		}
	case schema.AuthorizationRequired.String():
		// Empty country list stays required when the quote is valid.
		return schema.WorkAuthorization{
			Value:     schema.AuthorizationRequired.String(),
			Countries: filterCountries(a.Countries),
			Quote:     a.Quote,
		}
	default:
		return unknownWorkAuthorization()
	}
}

func coerceTimezone(t schema.TimezoneConstraint, vacancyMatchText string) schema.TimezoneConstraint {
	if t.Value == unknownAnswer || t.Value == "" {
		return unknownTimezone()
	}
	if !quoteOK(t.Quote, vacancyMatchText) {
		return unknownTimezone()
	}
	switch t.Value {
	case "open":
		return schema.TimezoneConstraint{
			Value: "open",
			Zones: emptyStrings(),
			Quote: t.Quote,
		}
	case schema.ScopeRestricted.String():
		zones := filterZones(t.Zones)
		if len(zones) == 0 {
			return unknownTimezone()
		}
		return schema.TimezoneConstraint{
			Value: schema.ScopeRestricted.String(),
			Zones: zones,
			Quote: t.Quote,
		}
	default:
		return unknownTimezone()
	}
}

func coerceCompensation(c schema.Compensation, vacancyMatchText string) schema.Compensation {
	if c.Unknown {
		return unknownCompensation()
	}
	if !quoteOK(c.Quote, vacancyMatchText) {
		return unknownCompensation()
	}
	code, ok := isoCurrency(c.Currency)
	if !ok {
		return unknownCompensation()
	}
	period := ""
	if _, err := schema.FromStringPayPeriod(c.Period); err == nil {
		period = c.Period
	}
	return schema.Compensation{
		Currency: code,
		Min:      c.Min,
		Max:      c.Max,
		Period:   period,
		Quote:    c.Quote,
		Unknown:  false,
	}
}

func coerceQuotedValue(q schema.QuotedValue, vacancyMatchText string) schema.QuotedValue {
	if q.Value == unknownAnswer || q.Value == "" {
		return schema.QuotedValue{Value: unknownAnswer, Quote: nil}
	}
	if !quoteOK(q.Quote, vacancyMatchText) {
		return schema.QuotedValue{Value: unknownAnswer, Quote: nil}
	}
	return schema.QuotedValue{Value: q.Value, Quote: q.Quote}
}

func quoteOK(quote *string, vacancyMatchText string) bool {
	if quote == nil {
		return false
	}
	return QuoteMatches(*quote, vacancyMatchText)
}

func filterCountries(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		code, ok := isoCountry(raw)
		if !ok {
			continue
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func filterRegions(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		r, err := schema.FromStringRegion(raw)
		if err != nil {
			continue
		}
		code := r.String()
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func filterZones(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		z, err := schema.FromStringTimezoneZone(raw)
		if err != nil {
			continue
		}
		code := z.String()
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func isoCountry(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if len(s) != 2 {
		return "", false
	}
	r, err := language.ParseRegion(s)
	if err != nil || !r.IsCountry() {
		return "", false
	}
	return r.String(), true
}

func isoCurrency(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	u, err := currency.ParseISO(s)
	if err != nil {
		return "", false
	}
	return u.String(), true
}

func emptyStrings() []string { return []string{} }

func unknownOffice() schema.Office {
	return schema.Office{
		Countries: emptyStrings(),
		Regions:   emptyStrings(),
		Quote:     nil,
		Unknown:   true,
	}
}

func unknownHiringScope() schema.HiringScope {
	return schema.HiringScope{
		Value:     unknownAnswer,
		Countries: emptyStrings(),
		Regions:   emptyStrings(),
		Quote:     nil,
	}
}

func unknownWorkAuthorization() schema.WorkAuthorization {
	return schema.WorkAuthorization{
		Value:     unknownAnswer,
		Countries: emptyStrings(),
		Quote:     nil,
	}
}

func unknownTimezone() schema.TimezoneConstraint {
	return schema.TimezoneConstraint{
		Value: unknownAnswer,
		Zones: emptyStrings(),
		Quote: nil,
	}
}

func unknownCompensation() schema.Compensation {
	return schema.Compensation{
		Currency: "",
		Min:      nil,
		Max:      nil,
		Period:   "",
		Quote:    nil,
		Unknown:  true,
	}
}
