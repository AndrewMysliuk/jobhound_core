// Package impl scores jobs against a search profile.
package impl

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// Outcome is the score of one job. It does not set user status or a run id.
type Outcome struct {
	Bucket  schema.Bucket
	Score   int
	Signals []schema.Signal
}

// Evaluate applies one profile to one job. It does not read storage or the network.
// The clock is time.Now. Score passes the run start via evaluateAt.
// A known posted_at older than PostedAtRejectDays rejects before phrase checks.
// A zero posted_at is not a reject. Title excludes run next, then text excludes
// against the title, the description, and the location line. Location countries
// and regions run after that. A hit stops the pass. Otherwise the bucket is PASSED,
// including a score of 0 or below.
func Evaluate(profile profileschema.Profile, job jobdata.Job) Outcome {
	return evaluateAt(profile, job, time.Now().UTC())
}

func evaluateAt(profile profileschema.Profile, job jobdata.Job, now time.Time) Outcome {
	if postedAtStale(job.PostedAt, now) {
		return rejected(schema.SignalCutPostedAt, job.PostedAt.UTC().Format(time.RFC3339))
	}
	if phrase, ok := firstMatch(profile.ExcludeTitle, job.Title); ok {
		return rejected(schema.SignalCutExcludeTitle, phrase)
	}
	text := jobText(job.Title, job.Description)
	excludeText := jobText(text, job.Location.Raw)
	if phrase, ok := firstMatch(profile.ExcludeText, excludeText); ok {
		return rejected(schema.SignalCutExcludeText, phrase)
	}
	if terms, ok := cutLocation(profile, job.Location); ok {
		return rejected(schema.SignalCutLocation, terms...)
	}

	var signals []schema.Signal
	score := 0
	for _, phrase := range matchingPhrases(profile.Queries, text) {
		signals = append(signals, schema.Signal{
			Code:   schema.SignalQuery,
			Points: schema.QueryPoints,
			Terms:  []string{phrase},
		})
		score += schema.QueryPoints
	}
	for _, phrase := range matchingPhrases(profile.Penalties, text) {
		signals = append(signals, schema.Signal{
			Code:   schema.SignalPenalty,
			Points: schema.PenaltyPoints,
			Terms:  []string{phrase},
		})
		score += schema.PenaltyPoints
	}
	return Outcome{Bucket: schema.BucketPassed, Score: score, Signals: signals}
}

func postedAtStale(posted, now time.Time) bool {
	if posted.IsZero() || now.IsZero() {
		return false
	}
	cutoff := now.UTC().Add(-time.Duration(schema.PostedAtRejectDays) * 24 * time.Hour)
	return posted.UTC().Before(cutoff)
}

func rejected(code schema.SignalCode, terms ...string) Outcome {
	return Outcome{
		Bucket: schema.BucketRejected,
		Signals: []schema.Signal{{
			Code:  code,
			Terms: terms,
		}},
	}
}

// cutLocation rejects a known hiring scope that misses the profile lists.
// Empty countries and regions stay. Empty profile lists turn the check off.
func cutLocation(profile profileschema.Profile, loc jobdata.Location) ([]string, bool) {
	allowedCountries := compact(profile.LocationCountries)
	allowedRegions := compact(profile.LocationRegions)
	if len(allowedCountries) == 0 && len(allowedRegions) == 0 {
		return nil, false
	}
	countries := compact(loc.Countries)
	regions := compact(loc.Regions)
	if len(countries) == 0 && len(regions) == 0 {
		return nil, false
	}
	if overlaps(allowedCountries, countries) || overlaps(allowedRegions, regions) {
		return nil, false
	}
	terms := make([]string, 0, len(countries)+len(regions))
	terms = append(terms, countries...)
	terms = append(terms, regions...)
	return terms, true
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

func overlaps(allowed, got []string) bool {
	for _, code := range got {
		for _, want := range allowed {
			if strings.EqualFold(want, code) {
				return true
			}
		}
	}
	return false
}

func jobText(title, description string) string {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	switch {
	case title == "":
		return description
	case description == "":
		return title
	default:
		return title + "\n" + description
	}
}

func firstMatch(phrases []string, text string) (string, bool) {
	for _, phrase := range phrases {
		if containsPhrase(text, phrase) {
			return strings.TrimSpace(phrase), true
		}
	}
	return "", false
}

func matchingPhrases(phrases []string, text string) []string {
	var hit []string
	for _, phrase := range phrases {
		if containsPhrase(text, phrase) {
			hit = append(hit, strings.TrimSpace(phrase))
		}
	}
	return hit
}

func containsPhrase(text, phrase string) bool {
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return false
	}
	hay := strings.ToLower(text)
	needle := strings.ToLower(phrase)
	if len(needle) > len(hay) {
		return false
	}
	for start := 0; start <= len(hay)-len(needle); {
		rel := strings.Index(hay[start:], needle)
		if rel < 0 {
			return false
		}
		abs := start + rel
		if wordBounded(hay, abs, len(needle)) {
			return true
		}
		start = abs + 1
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
