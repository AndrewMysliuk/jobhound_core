// Package sourceids is the allow-list of collector source ids for profile YAML.
package sourceids

import (
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/consider"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/getro"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/weworkremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
)

// KnownSourceIDs returns aggregator source ids, then Consider tenants, then Getro tenants.
func KnownSourceIDs() []string {
	ids := []string{
		europeremotely.SourceName,
		workingnomads.SourceName,
		himalayas.SourceName,
		remotifyeurope.SourceName,
		weworkremotely.SourceName,
		wellfound.SourceName,
		vuejobs.SourceName,
		golangcafe.SourceName,
		builtin.SourceName,
	}
	for _, tenant := range consider.Tenants() {
		ids = append(ids, tenant.SourceID)
	}
	for _, tenant := range getro.Tenants() {
		ids = append(ids, tenant.SourceID)
	}
	return ids
}
