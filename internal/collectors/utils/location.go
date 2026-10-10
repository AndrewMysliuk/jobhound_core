package utils

import (
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// LocationFromParsed fills schema.Location from values a collector has already parsed.
// countryCode is included when it is not already in countries. offsets become the timezone string.
func LocationFromParsed(remote *bool, countryCode string, countries, regions []string, raw string, offsets []float64) schema.Location {
	return schema.Location{
		Type:      locationType(remote),
		Countries: withCountryCode(countries, countryCode),
		Regions:   regions,
		Raw:       raw,
		Timezone:  formatTimezoneOffsets(offsets),
	}
}

func locationType(remote *bool) string {
	if remote == nil {
		return ""
	}
	if *remote {
		return schema.LocationRemote
	}
	return schema.LocationOffice
}

func withCountryCode(countries []string, code string) []string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return countries
	}
	for _, c := range countries {
		if strings.EqualFold(strings.TrimSpace(c), code) {
			return countries
		}
	}
	return append([]string{code}, countries...)
}

func formatTimezoneOffsets(offsets []float64) string {
	if len(offsets) == 0 {
		return ""
	}
	parts := make([]string, len(offsets))
	for i, n := range offsets {
		parts[i] = strconv.FormatFloat(n, 'f', -1, 64)
	}
	return strings.Join(parts, ",")
}
