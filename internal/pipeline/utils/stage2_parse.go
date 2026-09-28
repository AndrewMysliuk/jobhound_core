package utils

import (
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
)

// ParseListing extracts stage-2 fields from a job listing.
func ParseListing(j schema.Job) (pipelineschema.ParsedListing, error) {
	if err := loadGeoRegions(); err != nil {
		return pipelineschema.ParsedListing{}, err
	}
	var codes []string
	codes = append(codes, j.HiringCountries...)
	for _, region := range j.HiringRegions {
		region = strings.ToUpper(strings.TrimSpace(region))
		if region == "" || region == schema.RegionCodeWorldwide.String() {
			continue
		}
		if expanded, ok := ExpandRegionGroup(region); ok {
			codes = append(codes, expanded...)
		}
	}
	position := ""
	if j.Position != nil {
		position = strings.TrimSpace(*j.Position)
	}
	return pipelineschema.ParsedListing{
		CountriesAllowed: uniqueCountryCodes(codes),
		Position:         position,
	}, nil
}

func uniqueCountryCodes(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, c := range in {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}
