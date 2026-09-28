package utils

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	jobdata "github.com/andrewmysliuk/jobhound_core/data"
)

var (
	geoOnce   sync.Once
	geoGroups map[string][]string
	geoErr    error
)

func loadGeoRegions() error {
	geoOnce.Do(func() {
		var raw map[string][]string
		if err := json.Unmarshal(jobdata.GeoRegionsJSON, &raw); err != nil {
			geoErr = fmt.Errorf("pipeline utils: geo_regions.json: %w", err)
			return
		}
		geoGroups = make(map[string][]string, len(raw))
		for name, codes := range raw {
			geoGroups[strings.ToUpper(strings.TrimSpace(name))] = append([]string(nil), codes...)
		}
	})
	return geoErr
}

// ExpandRegionGroup returns ISO alpha-2 codes for a named group from geo_regions.json.
// Groups serve two roles: geography produced by collectors (RegionCode values such as EUROPE, APAC)
// and hiring policy used in rule values (EUROPE_HIRING_OK). The two may hold the same codes today
// and are still kept apart, because policy changes must not move a collector's region.
func ExpandRegionGroup(name string) ([]string, bool) {
	if err := loadGeoRegions(); err != nil {
		return nil, false
	}
	codes, ok := geoGroups[strings.ToUpper(strings.TrimSpace(name))]
	if !ok || len(codes) == 0 {
		return nil, false
	}
	return append([]string(nil), codes...), true
}
