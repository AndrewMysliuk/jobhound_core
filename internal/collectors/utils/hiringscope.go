package utils

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	jobschema "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

var hiringScopePartSplit = regexp.MustCompile(`(?i)\s+and\s+|[,/;|]`)

type hiringScopePhrase struct {
	phrase  string
	region  string
	country string
}

// regionPhrases are matched on whole words inside a location fragment.
var regionPhrases = []hiringScopePhrase{
	{phrase: "anywhere in the world", region: jobschema.RegionCodeWorldwide.String()},
	{phrase: "global remote", region: jobschema.RegionCodeWorldwide.String()},
	{phrase: "worldwide", region: jobschema.RegionCodeWorldwide.String()},
	{phrase: "anywhere", region: jobschema.RegionCodeWorldwide.String()},
	{phrase: "united states only", country: "US"},
	{phrase: "usa only", country: "US"},
	{phrase: "us only", country: "US"},
	{phrase: "north america", region: jobschema.RegionCodeNorthAmerica.String()},
	{phrase: "latin america", region: jobschema.RegionCodeLATAM.String()},
	{phrase: "americas", region: jobschema.RegionCodeLATAM.String()},
	{phrase: "latam", region: jobschema.RegionCodeLATAM.String()},
	{phrase: "asia pacific", region: jobschema.RegionCodeAPAC.String()},
	{phrase: "apac", region: jobschema.RegionCodeAPAC.String()},
	{phrase: "middle east", region: jobschema.RegionCodeMiddleEast.String()},
	{phrase: "gcc", region: jobschema.RegionCodeMiddleEast.String()},
	{phrase: "europe only", region: jobschema.RegionCodeEurope.String()},
	{phrase: "europe", region: jobschema.RegionCodeEurope.String()},
	{phrase: "eea", region: jobschema.RegionCodeEurope.String()},
	{phrase: "eu", region: jobschema.RegionCodeEurope.String()},
	{phrase: "emea", region: jobschema.RegionCodeEMEA.String()},
	{phrase: "africa", region: jobschema.RegionCodeAfrica.String()},
}

func init() {
	sort.SliceStable(regionPhrases, func(i, j int) bool {
		return len(regionPhrases[i].phrase) > len(regionPhrases[j].phrase)
	})
}

// ParseHiringScope resolves location restriction fragments into ISO countries and named regions.
// A fragment that names a country wins over a region phrase, so "South Africa" yields ZA, not AFRICA.
// joined is the trimmed concatenation of non-empty raw inputs (for HiringRaw).
func ParseHiringScope(r *CountryResolver, raw ...string) (countries []string, regions []string, joined string) {
	var parts []string
	var joinParts []string
	rawSeen := make(map[string]struct{})
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, dup := rawSeen[s]; !dup {
			rawSeen[s] = struct{}{}
			joinParts = append(joinParts, s)
		}
		for _, p := range hiringScopePartSplit.Split(s, -1) {
			if p = strings.TrimSpace(p); p != "" {
				parts = append(parts, p)
			}
		}
	}
	joined = strings.TrimSpace(strings.Join(joinParts, " "))

	countrySeen := make(map[string]struct{})
	regionSeen := make(map[string]struct{})
	addCountry := func(code string) {
		if code == "" {
			return
		}
		if _, dup := countrySeen[code]; dup {
			return
		}
		countrySeen[code] = struct{}{}
		countries = append(countries, code)
	}
	addRegion := func(region string) {
		if region == "" {
			return
		}
		if _, dup := regionSeen[region]; dup {
			return
		}
		regionSeen[region] = struct{}{}
		regions = append(regions, region)
	}

	for _, part := range parts {
		if code := resolveCountryFragment(r, part); code != "" {
			addCountry(code)
			continue
		}
		lower := strings.ToLower(part)
		for _, entry := range regionPhrases {
			if !containsWholeWordPhrase(lower, entry.phrase) {
				continue
			}
			if entry.country != "" {
				addCountry(entry.country)
				continue
			}
			addRegion(entry.region)
		}
	}
	return countries, regions, joined
}

// resolveCountryFragment tries the fragment as-is (keeps aliases like "u.s."), then without
// surrounding punctuation and emoji, so "🇺🇸 United States of America" also resolves.
func resolveCountryFragment(r *CountryResolver, part string) string {
	if code := r.Alpha2ForName(part); code != "" {
		return code
	}
	trimmed := strings.TrimFunc(part, func(ru rune) bool { return !isWordRune(ru) })
	if trimmed == part {
		return ""
	}
	return r.Alpha2ForName(trimmed)
}

func containsWholeWordPhrase(text, phrase string) bool {
	if phrase == "" {
		return false
	}
	for start := 0; start <= len(text)-len(phrase); {
		idx := strings.Index(text[start:], phrase)
		if idx < 0 {
			return false
		}
		abs := start + idx
		if wholeWordBoundary(text, abs, len(phrase)) {
			return true
		}
		start = abs + 1
	}
	return false
}

func wholeWordBoundary(text string, start, length int) bool {
	if start > 0 {
		if r, _ := utf8.DecodeLastRuneInString(text[:start]); isWordRune(r) {
			return false
		}
	}
	end := start + length
	if end < len(text) {
		if r, _ := utf8.DecodeRuneInString(text[end:]); isWordRune(r) {
			return false
		}
	}
	return true
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
