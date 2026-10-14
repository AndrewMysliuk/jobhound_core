package scoring_workflows

import (
	"errors"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
)

const (
	sourceWellfound         = "wellfound"
	sourceWeWorkRemotely    = "we_work_remotely"
	sourceVueJobs           = "vue_jobs"
	sourceGolangCafe        = "golang_cafe"
	sourceBuiltin           = "builtin"
	sourceEjobs             = "ejobs"
	sourceOlxRO             = "olx_ro"
	browserLane             = "browser"
	architectureBrowserLane = "architecture-browser"
	considerSourcePrefix    = "consider_"
	catalogChildSlug        = "catalog"
)

// WorkflowID is the Temporal id for one profile. A second start while it is open is rejected.
func WorkflowID(profileID string) string {
	return "profile-run-" + profileID
}

// IngestChildren expands a profile into ingest child inputs.
// Keyword sources, including getro_*, get one child per query.
// consider_* gets one child: Queries is the profile query list and Query is empty.
// Wellfound gets one child per role slug.
// Catalog sources get one child with an empty query; their queries are ignored.
// A source listed in Children gets one child per slug and does not copy Queries.
// The slug catalog is an empty Query.
func IngestChildren(profile profileschema.Profile) []ingestschema.IngestSourceInput {
	var out []ingestschema.IngestSourceInput
	for _, raw := range profile.Sources {
		id := strings.ToLower(strings.TrimSpace(raw))
		if id == "" {
			continue
		}
		switch {
		case id == sourceWellfound:
			for _, slug := range profile.WellfoundRoles {
				slug = strings.TrimSpace(slug)
				if slug == "" {
					continue
				}
				out = append(out, ingestschema.IngestSourceInput{SourceID: id, Query: slug})
			}
		case catalogSource(id):
			out = append(out, ingestschema.IngestSourceInput{SourceID: id})
		case strings.HasPrefix(id, considerSourcePrefix):
			out = append(out, ingestschema.IngestSourceInput{
				SourceID: id,
				Queries:  append([]string(nil), profile.Queries...),
			})
		case len(profile.Children[id]) > 0:
			out = append(out, childrenInputs(id, profile.Children[id])...)
		default:
			for _, query := range profile.Queries {
				query = strings.TrimSpace(query)
				if query == "" {
					continue
				}
				out = append(out, ingestschema.IngestSourceInput{SourceID: id, Query: query})
			}
		}
	}
	return out
}

func childrenInputs(sourceID string, slugs []string) []ingestschema.IngestSourceInput {
	out := make([]ingestschema.IngestSourceInput, 0, len(slugs))
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		if slug == "" {
			continue
		}
		child := ingestschema.IngestSourceInput{SourceID: sourceID}
		if slug != catalogChildSlug {
			child.Query = slug
		}
		out = append(out, child)
	}
	return out
}

// IngestLane is one resource that cannot be fetched concurrently with itself.
// Builtin and Golang Cafe share Chromium. ejobs and olx_ro share architecture-browser.
// Every other source is its own lane.
type IngestLane struct {
	Name     string
	Children []ingestschema.IngestSourceInput
}

// IngestLanes groups children so different resources run together and one resource stays in order.
func IngestLanes(profile profileschema.Profile) []IngestLane {
	var lanes []IngestLane
	index := map[string]int{}
	for _, child := range IngestChildren(profile) {
		name := resourceLane(child.SourceID)
		i, ok := index[name]
		if !ok {
			index[name] = len(lanes)
			lanes = append(lanes, IngestLane{Name: name})
			i = index[name]
		}
		lanes[i].Children = append(lanes[i].Children, child)
	}
	return lanes
}

// IngestChildWorkflowID names the child by the board and the query it fetches.
// Empty query is the catalog segment. Spaces become hyphens.
func IngestChildWorkflowID(in ingestschema.IngestSourceInput) string {
	source := ingest.NormalizeSourceID(in.SourceID)
	segment := ingest.NormalizeSourceID(in.Query)
	if segment == "" {
		segment = "catalog"
	}
	segment = strings.Join(strings.Fields(segment), "-")
	return "ingest-" + source + "-" + segment
}

func resourceLane(sourceID string) string {
	switch sourceID {
	case sourceBuiltin, sourceGolangCafe:
		return browserLane
	case sourceEjobs, sourceOlxRO:
		return architectureBrowserLane
	default:
		return sourceID
	}
}

func catalogSource(id string) bool {
	switch id {
	case sourceWeWorkRemotely, sourceVueJobs, sourceGolangCafe:
		return true
	default:
		return false
	}
}

// ChildSkipped reports a child that must not fail the profile run:
// lock held, cooldown, rate limit, a dead tenant, or a source this worker did not build.
func ChildSkipped(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ingest.ErrLockHeld) || errors.Is(err, ingest.ErrCooldownActive) {
		return true
	}
	if errors.Is(err, collectors.ErrTenantUnavailable) {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, strings.ToLower(ingest.ErrLockHeld.Error())) {
		return true
	}
	if strings.Contains(msg, strings.ToLower(ingest.ErrCooldownActive.Error())) {
		return true
	}
	if strings.Contains(msg, strings.ToLower(collectors.ErrTenantUnavailable.Error())) {
		return true
	}
	if strings.Contains(msg, "unknown source_id") {
		return true
	}
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "rate limit")
}
