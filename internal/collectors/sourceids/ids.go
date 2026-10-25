// Package sourceids is the allow-list of collector source ids for profile YAML.
package sourceids

import (
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/archgee"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/archinect"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/baunetz"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/bdla"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/bestjobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/competitionline"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/consider"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/dezeen"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/dou"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/ejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/getro"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/hipo"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/landscapeinstitute"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/riba"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/weworkremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/worldarchitects"
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
		dou.SourceName,
		builtin.SourceName,
		bestjobs.SourceName,
		hipo.SourceName,
		ejobs.SourceName,
		dezeen.SourceName,
		riba.SourceName,
		landscapeinstitute.SourceName,
		archgee.SourceName,
		archinect.SourceName,
		competitionline.SourceName,
		baunetz.SourceName,
		bdla.SourceName,
		worldarchitects.SourceName,
	}
	for _, tenant := range consider.Tenants() {
		ids = append(ids, tenant.SourceID)
	}
	for _, tenant := range getro.Tenants() {
		ids = append(ids, tenant.SourceID)
	}
	return ids
}
