// Package impl scores jobs against a search profile.
package impl

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

const (
	negationWindowRunes = 50
	// fullTimeHoursPerYear annualizes an hourly figure at 40 hours × 52 weeks.
	fullTimeHoursPerYear = 40 * 52
	europeOfficeRule     = "europe_office"
	europeOfficeDelta    = -2
	europeLocationRule   = "gate_location.europe"
)

// Europe office countries are fixed in the scorer. They are not a profile allow list.
var europeCountries = map[string]struct{}{
	"AT": {}, "BE": {}, "BG": {}, "CY": {}, "CZ": {}, "DE": {}, "DK": {}, "EE": {},
	"ES": {}, "FI": {}, "FR": {}, "GR": {}, "HR": {}, "HU": {}, "IE": {}, "IT": {},
	"LT": {}, "LU": {}, "LV": {}, "MT": {}, "NL": {}, "PL": {}, "PT": {}, "SE": {},
	"SI": {}, "SK": {}, "IS": {}, "LI": {}, "NO": {}, "CH": {}, "GB": {}, "RO": {},
}

// Relocation phrases cancel the Europe office penalty. They are not a YAML group.
var relocationPhrases = []string{
	"relocation",
	"relocation package",
	"relocation assistance",
	"relocate",
	"relocare",
	"umzug",
	"umzugskosten",
}

var (
	salaryLeadCurrency     = regexp.MustCompile(`(?i)(€|\$|£|\busd\b|\beur\b|\bgbp\b|us\$)\s*(\d{1,3}(?:[, ]\d{3})+|\d+(?:\.\d+)?)(\s*[kK])?(?:\s*(?:-|\bto\b)\s*(?:€|\$|£|\busd\b|\beur\b|\bgbp\b|us\$)?\s*(\d{1,3}(?:[, ]\d{3})+|\d+(?:\.\d+)?)(\s*[kK])?)?`)
	salaryTrailCurrency    = regexp.MustCompile(`(?i)(\d{1,3}(?:[, ]\d{3})+|\d+(?:\.\d+)?)(\s*[kK])?(?:\s*(?:-|\bto\b)\s*(\d{1,3}(?:[, ]\d{3})+|\d+(?:\.\d+)?)(\s*[kK])?)?\s*\+?\s*-?\s*(€|\$|£|\busd\b|\beur\b|\bgbp\b|us\$)`)
	salaryPeriod           = regexp.MustCompile(`(?i)^\s*(?:/|\bper\b|\ba\b)?\s*(yearly|annual(?:ly)?|monthly|weekly|hourly|daily|year|month|week|hour|day)\b`)
	salaryMagnitude        = regexp.MustCompile(`(?i)^\s*(?:million|billion|mm|mn|bn|m|b)\b`)
	experienceYearsRO      = regexp.MustCompile(`(?i)(\d+)\s*\+?\s*ani\b`)
	experienceYearsEN      = regexp.MustCompile(`(?i)(\d+)\s*\+?\s*years?\b`)
	experienceYearsRange   = regexp.MustCompile(`(?i)(\d+)\s*[-–]\s*(\d+)\s*\+?\s*(?:years?|ani)\b`)
	juniorLevelDisjunction = regexp.MustCompile(`(?i)\b(?:junior|senior|principal|lead|jr\.?|sr\.?)(?:\s*(?:/|[-–—]|\bor\b|\bto\b)\s*(?:junior|senior|principal|lead|jr\.?|sr\.?))+`)
)

var foldDiacritics = strings.NewReplacer(
	"ă", "a", "â", "a", "î", "i", "ș", "s", "ț", "t",
	"Ă", "A", "Â", "A", "Î", "I", "Ș", "S", "Ț", "T",
	"ş", "s", "ţ", "t", "Ş", "S", "Ţ", "T",
)

const (
	architectureMaxExperienceYears = 3
	wrongDomainPenaltyName         = "wrong_domain"
)

// Outcome is the score of one job. It does not set user status or a run id.
type Outcome struct {
	Bucket  schema.Bucket
	Score   int
	Signals []schema.Signal
}

// Evaluate applies one profile to one job. It does not read storage or the network.
// The clock is time.Now. Score passes the run start via evaluateAt.
// All five steps run. The bucket is chosen only after the signals are collected.
func Evaluate(profile profileschema.Profile, job jobdata.Job) Outcome {
	return evaluateAt(profile, job, time.Now().UTC())
}

func evaluateAt(profile profileschema.Profile, job jobdata.Job, now time.Time) Outcome {
	var signals []schema.Signal
	signals = append(signals, matchGateRole(profile.GateRole, job.Title)...)
	signals = append(signals, matchPhrases(profile.GateHard, true, "gate_hard", schema.SignalGateHard, job.Title, job.Description, job.Location.Raw)...)
	signals = append(signals, experienceYearsGate(profile, job)...)
	signals = append(signals, matchGateLocation(profile, job, now)...)
	signals = append(signals, matchSignalsPlus(profile, job, now)...)
	signals = append(signals, matchPenaltiesSoft(profile, job)...)
	score := scoreOf(signals)
	return Outcome{
		Bucket:  chooseBucket(profile, score, signals),
		Score:   score,
		Signals: signals,
	}
}

// PostedAtRejectDaysForProfile returns the stale posted_at cutoff for scoring this profile.
func PostedAtRejectDaysForProfile(profile profileschema.Profile) int {
	if profile.Domain == profileschema.DomainArchitecture {
		return schema.PostedAtRejectDaysArchitecture
	}
	return schema.PostedAtRejectDays
}

func matchSignalsPlus(profile profileschema.Profile, job jobdata.Job, now time.Time) []schema.Signal {
	plus := profile.SignalsPlus
	var signals []schema.Signal
	signals = append(signals, matchTermSignals(plus.Title, "signals_plus.title", 0, false, job.Title)...)
	signals = append(signals, matchTermSignals(plus.Text.Terms, "signals_plus.text", plus.Text.MaxTotal, true, job.Description, job.Location.Raw)...)
	if signal, ok := salarySignal(plus.Salary, job); ok {
		signals = append(signals, signal)
	}
	if signal, ok := freshnessSignal(plus.Freshness, job, now); ok {
		signals = append(signals, signal)
	}
	return signals
}

func matchTermSignals(terms map[string]int, rule string, cap int, applyCap bool, fields ...string) []schema.Signal {
	if len(terms) == 0 {
		return nil
	}
	keys := make([]string, 0, len(terms))
	for term := range terms {
		keys = append(keys, term)
	}
	sort.Strings(keys)
	remaining := cap
	var signals []schema.Signal
	for _, term := range keys {
		if strings.TrimSpace(term) == "" {
			continue
		}
		n := 0
		for _, field := range fields {
			n += activeHits(field, term, false)
		}
		if n == 0 {
			continue
		}
		awarded := terms[term]
		if applyCap {
			awarded = clipBodyAward(awarded, &remaining)
		}
		signals = append(signals, schema.Signal{
			Code:        schema.SignalPlus,
			Rule:        rule,
			Phrase:      term,
			Delta:       awarded,
			Occurrences: n,
		})
	}
	return signals
}

// clipBodyAward returns the points a body term actually adds.
// A positive weight is cut so the running total stays within max_total.
// Once the cap is spent the term still fires with delta 0.
func clipBodyAward(weight int, remaining *int) int {
	if weight <= 0 {
		return weight
	}
	if *remaining <= 0 {
		return 0
	}
	if weight > *remaining {
		weight = *remaining
	}
	*remaining -= weight
	return weight
}

func salarySignal(cfg profileschema.SalarySignals, job jobdata.Job) (schema.Signal, bool) {
	if !salaryConfigured(cfg) {
		return schema.Signal{}, false
	}
	// The raw compensation field wins. Title, then description, are fallbacks.
	for _, text := range []string{job.SalaryRaw, job.Title, job.Description} {
		yearly, currency, ok := parseSalaryYearly(text)
		if !ok {
			continue
		}
		delta, ok := salaryDelta(cfg, yearly, currency)
		if !ok {
			continue
		}
		return schema.Signal{
			Code:  schema.SignalPlus,
			Rule:  "signals_plus.salary",
			Delta: delta,
		}, true
	}
	return schema.Signal{}, false
}

// salaryConfigured reports whether the profile sets a salary rule.
// An omitted block is all zeros and does not score.
func salaryConfigured(cfg profileschema.SalarySignals) bool {
	return cfg.MinEURYear != 0 || cfg.MinUSDYear != 0 || cfg.BonusIfAbove != 0 || cfg.PenaltyIfBelow != 0
}

// salaryDelta applies BonusIfAbove or PenaltyIfBelow as stored.
// The minimum is inclusive. A zero minimum on the posting's currency
// defers to the other minimum after USDPerEUR.
func salaryDelta(cfg profileschema.SalarySignals, yearly float64, currency string) (int, bool) {
	comparable, threshold, ok := salaryComparable(cfg, yearly, currency)
	if !ok {
		return 0, false
	}
	if int(math.Round(comparable)) >= threshold {
		return cfg.BonusIfAbove, true
	}
	return cfg.PenaltyIfBelow, true
}

func salaryComparable(cfg profileschema.SalarySignals, yearly float64, currency string) (float64, int, bool) {
	switch currency {
	case "EUR":
		if cfg.MinEURYear != 0 || cfg.MinUSDYear == 0 {
			return yearly, cfg.MinEURYear, true
		}
		if cfg.USDPerEUR <= 0 {
			return 0, 0, false
		}
		return yearly * cfg.USDPerEUR, cfg.MinUSDYear, true
	case "USD":
		if cfg.MinUSDYear != 0 || cfg.MinEURYear == 0 {
			return yearly, cfg.MinUSDYear, true
		}
		if cfg.USDPerEUR <= 0 {
			return 0, 0, false
		}
		return yearly / cfg.USDPerEUR, cfg.MinEURYear, true
	default:
		return 0, 0, false
	}
}

type salarySpan struct {
	end      int
	upper    float64
	currency string
}

func parseSalaryYearly(text string) (float64, string, bool) {
	text = normalizeSalaryText(text)
	for len(text) > 0 {
		span, ok := earliestSalary(text)
		if !ok || span.end <= 0 {
			return 0, "", false
		}
		rest := text[span.end:]
		text = rest
		if salaryRejected(rest) {
			continue
		}
		currency := canonicalCurrency(span.currency)
		if currency == "" {
			continue
		}
		yearly := span.upper * salaryMultiplier(rest)
		if yearly < 1000 {
			continue
		}
		return yearly, currency, true
	}
	return 0, "", false
}

func earliestSalary(text string) (salarySpan, bool) {
	lead := salaryLeadCurrency.FindStringSubmatchIndex(text)
	trail := salaryTrailCurrency.FindStringSubmatchIndex(text)
	switch {
	case lead == nil && trail == nil:
		return salarySpan{}, false
	case trail == nil || (lead != nil && lead[0] <= trail[0]):
		return spanFromLead(text, lead), true
	default:
		return spanFromTrail(text, trail), true
	}
}

func spanFromLead(text string, idx []int) salarySpan {
	first, ok := parseAmountToken(submatch(text, idx, 2), submatch(text, idx, 3))
	if !ok {
		return salarySpan{end: idx[1]}
	}
	second, secondOK := parseAmountToken(submatch(text, idx, 4), submatch(text, idx, 5))
	return salarySpan{
		end:      idx[1],
		upper:    upperOf(first, second, secondOK),
		currency: submatch(text, idx, 1),
	}
}

func spanFromTrail(text string, idx []int) salarySpan {
	first, ok := parseAmountToken(submatch(text, idx, 1), submatch(text, idx, 2))
	if !ok {
		return salarySpan{end: idx[1]}
	}
	second, secondOK := parseAmountToken(submatch(text, idx, 3), submatch(text, idx, 4))
	return salarySpan{
		end:      idx[1],
		upper:    upperOf(first, second, secondOK),
		currency: submatch(text, idx, 5),
	}
}

func submatch(text string, idx []int, group int) string {
	start, end := idx[group*2], idx[group*2+1]
	if start < 0 || end < 0 || start > end || end > len(text) {
		return ""
	}
	return text[start:end]
}

func parseAmountToken(raw, k string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	raw = strings.ReplaceAll(raw, ",", "")
	raw = strings.ReplaceAll(raw, " ", "")
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || n < 0 {
		return 0, false
	}
	if strings.ContainsAny(k, "kK") {
		n *= 1000
	}
	return n, true
}

func upperOf(first, second float64, secondOK bool) float64 {
	if secondOK && second > first {
		return second
	}
	return first
}

func canonicalCurrency(token string) string {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "€", "eur":
		return "EUR"
	case "$", "usd", "us$":
		return "USD"
	default:
		return ""
	}
}

func salaryRejected(rest string) bool {
	if salaryMagnitude.MatchString(rest) {
		return true
	}
	if rest == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return unicode.IsLetter(r)
}

func salaryMultiplier(rest string) float64 {
	m := salaryPeriod.FindStringSubmatch(rest)
	if m == nil {
		return 1
	}
	switch strings.ToLower(m[1]) {
	case "month", "monthly":
		return 12
	case "week", "weekly":
		return 52
	case "day", "daily":
		return 5 * 52
	case "hour", "hourly":
		return fullTimeHoursPerYear
	default:
		return 1
	}
}

func normalizeSalaryText(text string) string {
	return strings.NewReplacer(
		"\u2013", "-",
		"\u2014", "-",
		"\u2212", "-",
		"\u00a0", " ",
	).Replace(text)
}

func freshnessSignal(fresh profileschema.FreshnessSignals, job jobdata.Job, now time.Time) (schema.Signal, bool) {
	ts, ok := freshnessClock(job)
	if !ok {
		return schema.Signal{}, false
	}
	age := now.Sub(ts)
	var delta int
	switch {
	case age <= 3*24*time.Hour:
		delta = fresh.Days3
	case age <= 7*24*time.Hour:
		delta = fresh.Days7
	default:
		return schema.Signal{}, false
	}
	if delta == 0 {
		return schema.Signal{}, false
	}
	return schema.Signal{
		Code:  schema.SignalPlus,
		Rule:  "signals_plus.freshness",
		Delta: delta,
	}, true
}

func freshnessClock(job jobdata.Job) (time.Time, bool) {
	if !job.PostedAt.IsZero() {
		return job.PostedAt, true
	}
	if !job.FirstSeenAt.IsZero() {
		return job.FirstSeenAt, true
	}
	return time.Time{}, false
}

func matchPenaltiesSoft(profile profileschema.Profile, job jobdata.Job) []schema.Signal {
	var signals []schema.Signal
	for _, group := range profile.PenaltiesSoft {
		signal, ok := penaltySignal(group, job)
		if !ok {
			continue
		}
		signals = append(signals, signal)
	}
	return signals
}

// penaltySignal fires a group once, on the first term in slice order that qualifies.
// A title hit qualifies even when the body is under the threshold.
// Body hits qualify only at the threshold. MinOccurrences below 1 means one hit.
// Occurrences is the count that met that bar. Negation runs only when the group asks.
// wrong_domain counts distinct matching terms; MinOccurrences is the distinct-term threshold.
func penaltySignal(group profileschema.PenaltyGroup, job jobdata.Job) (schema.Signal, bool) {
	if group.Name == wrongDomainPenaltyName {
		return wrongDomainPenalty(group, job)
	}
	need := group.MinOccurrences
	if need < 1 {
		need = 1
	}
	for _, term := range group.Terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		titleHits := activeHits(job.Title, term, group.CheckNegation)
		bodyHits := activeHits(job.Description, term, group.CheckNegation)
		occurrences, ok := penaltyOccurrences(titleHits, bodyHits, need)
		if !ok {
			continue
		}
		return schema.Signal{
			Code:        schema.SignalPenalty,
			Rule:        group.Name,
			Phrase:      term,
			Delta:       group.Weight,
			Occurrences: occurrences,
		}, true
	}
	return schema.Signal{}, false
}

func wrongDomainPenalty(group profileschema.PenaltyGroup, job jobdata.Job) (schema.Signal, bool) {
	need := group.MinOccurrences
	if need < 2 {
		need = 2
	}
	var matched []string
	for _, term := range group.Terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if activeHits(job.Title, term, group.CheckNegation) == 0 &&
			activeHits(job.Description, term, group.CheckNegation) == 0 {
			continue
		}
		matched = append(matched, term)
	}
	if len(matched) < need {
		return schema.Signal{}, false
	}
	return schema.Signal{
		Code:        schema.SignalPenalty,
		Rule:        group.Name,
		Phrase:      matched[0],
		Delta:       group.Weight,
		Occurrences: len(matched),
	}, true
}

func experienceYearsGate(profile profileschema.Profile, job jobdata.Job) []schema.Signal {
	if profile.Domain != profileschema.DomainArchitecture {
		return nil
	}
	text := job.Title + "\n" + job.Description
	maxYears := maxExperienceYears(text)
	if maxYears <= architectureMaxExperienceYears {
		return nil
	}
	return []schema.Signal{{
		Code:        schema.SignalGateHard,
		Rule:        "experience_years",
		Phrase:      strconv.Itoa(maxYears),
		Occurrences: 1,
	}}
}

func maxExperienceYears(text string) int {
	folded := foldMatchText(text)
	max := 0
	for _, sub := range experienceYearsRange.FindAllStringSubmatch(folded, -1) {
		max = maxInt(max, atoi(sub[1]), atoi(sub[2]))
	}
	for _, sub := range experienceYearsRO.FindAllStringSubmatch(folded, -1) {
		max = maxInt(max, atoi(sub[1]))
	}
	for _, sub := range experienceYearsEN.FindAllStringSubmatch(folded, -1) {
		max = maxInt(max, atoi(sub[1]))
	}
	return max
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func maxInt(values ...int) int {
	m := 0
	for _, v := range values {
		if v > m {
			m = v
		}
	}
	return m
}

func foldMatchText(s string) string {
	return foldDiacritics.Replace(strings.ToLower(s))
}

func penaltyOccurrences(titleHits, bodyHits, need int) (int, bool) {
	if titleHits > 0 && bodyHits < need {
		return titleHits, true
	}
	if bodyHits >= need {
		return bodyHits, true
	}
	return 0, false
}

func matchGateLocation(profile profileschema.Profile, job jobdata.Job, now time.Time) []schema.Signal {
	gate := profile.GateLocation
	var signals []schema.Signal
	if !remoteLocation(job.Location) {
		signals = append(signals, europeTier(job)...)
		signals = append(signals, locationCuts(gate, job.Location)...)
	}
	signals = append(signals, matchPhrases(gate.RejectTimezones, true, "gate_location.reject_timezones", schema.SignalCutTimezone, job.Title, job.Description, job.Location.Raw, job.Location.Timezone)...)
	signals = append(signals, postedAtCut(job.PostedAt, now, PostedAtRejectDaysForProfile(profile))...)
	return signals
}

func remoteLocation(loc jobdata.Location) bool {
	return strings.EqualFold(strings.TrimSpace(loc.Type), jobdata.LocationRemote)
}

// europeTier applies the fixed Europe office rule before the profile allow lists.
// A remote job never reaches it. An empty location adds nothing.
// A country outside the Europe set is CUT_LOCATION. An office job whose countries
// are all in that set, and none of them is RO, takes one −2 unless the posting offers relocation.
func europeTier(job jobdata.Job) []schema.Signal {
	countries := compact(job.Location.Countries)
	regions := compact(job.Location.Regions)
	if len(countries) == 0 && len(regions) == 0 {
		return nil
	}
	var signals []schema.Signal
	outside := false
	hasRO := false
	for _, code := range countries {
		if !europeCountry(code) {
			outside = true
			signals = append(signals, schema.Signal{
				Code:   schema.SignalCutLocation,
				Rule:   europeLocationRule,
				Phrase: code,
			})
			continue
		}
		if strings.EqualFold(code, "RO") {
			hasRO = true
		}
	}
	if outside || len(countries) == 0 || hasRO || relocationOffered(job) {
		return signals
	}
	return append(signals, schema.Signal{
		Code:  schema.SignalPenalty,
		Rule:  europeOfficeRule,
		Delta: europeOfficeDelta,
	})
}

func europeCountry(code string) bool {
	_, ok := europeCountries[strings.ToUpper(strings.TrimSpace(code))]
	return ok
}

func relocationOffered(job jobdata.Job) bool {
	for _, phrase := range relocationPhrases {
		if activeHits(job.Title, phrase, false) > 0 || activeHits(job.Description, phrase, false) > 0 {
			return true
		}
	}
	return false
}

func locationCuts(gate profileschema.GateLocation, loc jobdata.Location) []schema.Signal {
	allowedCountries := compact(gate.AllowCountries)
	allowedRegions := compact(gate.AllowRegions)
	if len(allowedCountries) == 0 && len(allowedRegions) == 0 {
		return nil
	}
	countries := compact(loc.Countries)
	regions := compact(loc.Regions)
	if len(countries) == 0 && len(regions) == 0 {
		return nil
	}
	var signals []schema.Signal
	for _, code := range countries {
		// The Europe tier already cut countries outside the set.
		if !europeCountry(code) {
			continue
		}
		if missesAllowLists(code, allowedCountries, allowedRegions) {
			signals = append(signals, schema.Signal{
				Code:   schema.SignalCutLocation,
				Rule:   "gate_location.allow_countries",
				Phrase: code,
			})
		}
	}
	for _, code := range regions {
		if missesAllowLists(code, allowedCountries, allowedRegions) {
			signals = append(signals, schema.Signal{
				Code:   schema.SignalCutLocation,
				Rule:   "gate_location.allow_regions",
				Phrase: code,
			})
		}
	}
	return signals
}

func missesAllowLists(code string, countries, regions []string) bool {
	return !containsFold(countries, code) && !containsFold(regions, code)
}

func postedAtCut(posted, now time.Time, rejectDays int) []schema.Signal {
	if !postedAtStale(posted, now, rejectDays) {
		return nil
	}
	return []schema.Signal{{
		Code:   schema.SignalCutPostedAt,
		Rule:   "posted_at",
		Phrase: posted.UTC().Format(time.RFC3339),
	}}
}

func postedAtStale(posted, now time.Time, rejectDays int) bool {
	if posted.IsZero() || now.IsZero() || rejectDays < 1 {
		return false
	}
	cutoff := now.UTC().Add(-time.Duration(rejectDays) * 24 * time.Hour)
	return posted.UTC().Before(cutoff)
}

func chooseBucket(profile profileschema.Profile, score int, signals []schema.Signal) schema.Bucket {
	for _, signal := range signals {
		if isGate(signal.Code) {
			return schema.BucketRejected
		}
	}
	top := profile.Buckets.Top
	if score >= top {
		if profile.Domain == profileschema.DomainArchitecture && !hasTitlePlusSignal(signals) {
			return schema.BucketReview
		}
		return schema.BucketTop
	}
	return schema.BucketReview
}

func hasTitlePlusSignal(signals []schema.Signal) bool {
	for _, signal := range signals {
		if signal.Code == schema.SignalPlus && strings.HasPrefix(signal.Rule, "signals_plus.title") {
			return true
		}
	}
	return false
}

func isGate(code schema.SignalCode) bool {
	switch code {
	case schema.SignalGateRole, schema.SignalGateHard, schema.SignalCutLocation, schema.SignalCutTimezone, schema.SignalCutPostedAt:
		return true
	default:
		return false
	}
}

func scoreOf(signals []schema.Signal) int {
	score := 0
	for _, signal := range signals {
		score += signal.Delta
	}
	return score
}

func matchGateRole(phrases []string, title string) []schema.Signal {
	spans := juniorLevelSpans(title)
	var signals []schema.Signal
	for _, phrase := range phrases {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			continue
		}
		var n int
		if levelGatePhrase(phrase) {
			n = countHits(title, phrase, false, func(start, length int) bool {
				end := start + length
				for _, span := range spans {
					if start >= span[0] && end <= span[1] {
						return true
					}
				}
				return false
			})
		} else {
			n = activeHits(title, phrase, false)
		}
		if n == 0 {
			continue
		}
		signals = append(signals, schema.Signal{
			Code:        schema.SignalGateRole,
			Rule:        "gate_role",
			Phrase:      phrase,
			Occurrences: n,
		})
	}
	return signals
}

func levelGatePhrase(phrase string) bool {
	switch foldMatchText(phrase) {
	case "senior", "sr.", "lead", "principal":
		return true
	default:
		return false
	}
}

// juniorLevelSpans returns chains like "junior or senior" and "junior/senior/lead".
// A chain without junior is left for the level gates.
func juniorLevelSpans(title string) [][2]int {
	folded := foldMatchText(title)
	var spans [][2]int
	for _, loc := range juniorLevelDisjunction.FindAllStringIndex(folded, -1) {
		span := folded[loc[0]:loc[1]]
		if activeHits(span, "junior", false) == 0 && activeHits(span, "jr", false) == 0 && activeHits(span, "jr.", false) == 0 {
			continue
		}
		spans = append(spans, [2]int{loc[0], loc[1]})
	}
	return spans
}

func matchPhrases(phrases []string, checkNegation bool, rule string, code schema.SignalCode, fields ...string) []schema.Signal {
	var signals []schema.Signal
	for _, phrase := range phrases {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			continue
		}
		n := 0
		for _, field := range fields {
			n += activeHits(field, phrase, checkNegation)
		}
		if n == 0 {
			continue
		}
		signals = append(signals, schema.Signal{
			Code:        code,
			Rule:        rule,
			Phrase:      phrase,
			Occurrences: n,
		})
	}
	return signals
}

func activeHits(text, phrase string, checkNegation bool) int {
	return countHits(text, phrase, checkNegation, nil)
}

func countHits(text, phrase string, checkNegation bool, skip func(start, length int) bool) int {
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return 0
	}
	hay := foldMatchText(text)
	needle := foldMatchText(phrase)
	if len(needle) > len(hay) {
		return 0
	}
	n := 0
	for start := 0; start <= len(hay)-len(needle); {
		rel := strings.Index(hay[start:], needle)
		if rel < 0 {
			return n
		}
		abs := start + rel
		if wordBounded(hay, abs, len(needle)) && (skip == nil || !skip(abs, len(needle))) && (!checkNegation || !negated(hay, abs)) {
			n++
		}
		start = abs + 1
	}
	return n
}

// negated reports whether the hit at start should be ignored.
// The window is the 50 characters before the phrase, clamped at the previous
// sentence boundary so a negation in an earlier sentence does not disarm it.
func negated(hay string, start int) bool {
	if strings.HasSuffix(hay[:start], "non-") {
		return true
	}
	return negationIn(negationWindow(hay, start))
}

func negationWindow(hay string, start int) string {
	prefix := []rune(hay[:start])
	if len(prefix) > negationWindowRunes {
		prefix = prefix[len(prefix)-negationWindowRunes:]
	}
	window := string(prefix)
	if cut := strings.LastIndexAny(window, ".!?"); cut >= 0 {
		window = window[cut+1:]
	}
	return window
}

func negationIn(window string) bool {
	if strings.Contains(window, "n't") {
		return true
	}
	return activeHits(window, "not", false) > 0 || activeHits(window, "no", false) > 0
}

func compact(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func containsFold(values []string, code string) bool {
	for _, value := range values {
		if strings.EqualFold(value, code) {
			return true
		}
	}
	return false
}

func wordBounded(text string, start, length int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:start])
		if isWordRune(r) {
			return false
		}
	}
	end := start + length
	if end < len(text) {
		r, _ := utf8.DecodeRuneInString(text[end:])
		if isWordRune(r) {
			return false
		}
	}
	return true
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
