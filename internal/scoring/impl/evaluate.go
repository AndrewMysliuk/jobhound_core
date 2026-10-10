// Package impl scores jobs against a search profile.
package impl

import (
	"strings"
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
// Title excludes run first, then text excludes. A hit stops the pass.
// Otherwise the bucket is PASSED, including a score of 0 or below.
func Evaluate(profile profileschema.Profile, job jobdata.Job) Outcome {
	if phrase, ok := firstMatch(profile.ExcludeTitle, job.Title); ok {
		return rejected(schema.SignalCutExcludeTitle, phrase)
	}
	text := jobText(job.Title, job.Description)
	if phrase, ok := firstMatch(profile.ExcludeText, text); ok {
		return rejected(schema.SignalCutExcludeText, phrase)
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

func rejected(code schema.SignalCode, phrase string) Outcome {
	return Outcome{
		Bucket: schema.BucketRejected,
		Signals: []schema.Signal{{
			Code:  code,
			Terms: []string{phrase},
		}},
	}
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
