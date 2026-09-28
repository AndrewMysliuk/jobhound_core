package utils

import (
	"strings"
	"unicode"
)

func normalizePhraseText(s string) string {
	s = strings.ToLower(s)
	return strings.ReplaceAll(s, "-", " ")
}

func hasLetterDigitBoundary(text string, start, length int) bool {
	if start < 0 || start+length > len(text) {
		return false
	}
	if start > 0 {
		if r, _ := utf8RuneAt(text, start-1); isLetterOrDigit(r) {
			return false
		}
	}
	end := start + length
	if end < len(text) {
		if r, _ := utf8RuneAt(text, end); isLetterOrDigit(r) {
			return false
		}
	}
	return true
}

func utf8RuneAt(s string, i int) (rune, int) {
	for _, r := range s[i:] {
		return r, 1
	}
	return 0, 0
}

func isLetterOrDigit(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func phraseMatchesWithNegation(text string, values []string, negationWindow int) (bool, string) {
	norm := normalizePhraseText(text)
	words := strings.Fields(norm)
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		needle := normalizePhraseText(v)
		if phraseHitInText(norm, words, needle, negationWindow) {
			return true, v
		}
	}
	return false, ""
}

func phraseHitInText(norm string, words []string, needle string, negationWindow int) bool {
	if needle == "" {
		return false
	}
	start := 0
	for {
		idx := strings.Index(norm[start:], needle)
		if idx < 0 {
			return false
		}
		abs := start + idx
		if !hasLetterDigitBoundary(norm, abs, len(needle)) {
			start = abs + 1
			continue
		}
		if negationWindow > 0 && negatedBeforePhrase(words, norm, abs, negationWindow) {
			start = abs + 1
			continue
		}
		return true
	}
}

func negatedBeforePhrase(words []string, norm string, phraseStart int, window int) bool {
	prefix := strings.TrimSpace(norm[:phraseStart])
	if prefix == "" {
		return false
	}
	prefixWords := strings.Fields(prefix)
	if len(prefixWords) == 0 {
		return false
	}
	start := len(prefixWords) - window
	if start < 0 {
		start = 0
	}
	for i := start; i < len(prefixWords); i++ {
		switch prefixWords[i] {
		case "no", "not", "without", "zero":
			return true
		}
	}
	return false
}

func expandCountryRuleValues(values []string) []string {
	var out []string
	for _, v := range values {
		v = strings.ToUpper(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		if expanded, ok := ExpandRegionGroup(v); ok {
			out = append(out, expanded...)
			continue
		}
		out = append(out, v)
	}
	return uniqueCountryCodes(out)
}

func countryAnyMatch(countries, values []string) (string, bool) {
	set := make(map[string]struct{}, len(countries))
	for _, c := range countries {
		set[strings.ToUpper(c)] = struct{}{}
	}
	for _, v := range expandCountryRuleValues(values) {
		if _, ok := set[v]; ok {
			return v, true
		}
	}
	return "", false
}

func countryExcludesMatch(countries, values []string) (string, bool) {
	if m, ok := countryAnyMatch(countries, values); ok {
		return m, false
	}
	if len(countries) > 0 {
		return strings.Join(countries, ","), true
	}
	return "", true
}
