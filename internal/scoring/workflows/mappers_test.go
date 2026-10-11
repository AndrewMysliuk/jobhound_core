package scoring_workflows

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	profileschema "github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
)

func TestIngestChildren_considerUnionGetroKeyword(t *testing.T) {
	profile := profileschema.Profile{
		Sources: []string{
			"consider_andreessen_horowitz",
			"consider_sequoia_capital",
			"getro_accel",
			"wellfound",
			"we_work_remotely",
			"vue_jobs",
			"golang_cafe",
		},
		Queries:        []string{"vue", "golang"},
		WellfoundRoles: []string{"frontend-engineer"},
	}

	got := IngestChildren(profile)
	want := []ingestschema.IngestSourceInput{
		{SourceID: "consider_andreessen_horowitz", Queries: []string{"vue", "golang"}},
		{SourceID: "consider_sequoia_capital", Queries: []string{"vue", "golang"}},
		{SourceID: "getro_accel", Query: "vue"},
		{SourceID: "getro_accel", Query: "golang"},
		{SourceID: "wellfound", Query: "frontend-engineer"},
		{SourceID: "we_work_remotely"},
		{SourceID: "vue_jobs"},
		{SourceID: "golang_cafe"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %#v, want %#v", got, want)
	}

	if id := IngestChildWorkflowID(got[0]); id != "ingest-consider_andreessen_horowitz-catalog" {
		t.Fatalf("workflow id = %q", id)
	}

	lanes := IngestLanes(profile)
	a16z := laneNamed(t, lanes, "consider_andreessen_horowitz")
	sequoia := laneNamed(t, lanes, "consider_sequoia_capital")
	if len(a16z.Children) != 1 || len(sequoia.Children) != 1 {
		t.Fatalf("consider lanes = %d and %d children, want 1 and 1", len(a16z.Children), len(sequoia.Children))
	}
	accel := laneNamed(t, lanes, "getro_accel")
	if !reflect.DeepEqual(accel.Children, want[2:4]) {
		t.Fatalf("getro lane = %#v, want vue then golang", accel.Children)
	}
}

func TestChildSkipped_tenantUnavailable(t *testing.T) {
	if !ChildSkipped(collectors.ErrTenantUnavailable) {
		t.Fatal("sentinel did not skip")
	}
	if !ChildSkipped(errors.New("collector tenant unavailable")) {
		t.Fatal("text-only tenant error did not skip")
	}
	if !ChildSkipped(errors.New("status 429")) {
		t.Fatal("429 did not skip")
	}
	if ChildSkipped(errors.New("ingest activity: collector does not implement union fetch")) {
		t.Fatal("missing FetchUnion skipped")
	}
}

func laneNamed(t *testing.T, lanes []IngestLane, name string) IngestLane {
	t.Helper()
	for _, lane := range lanes {
		if lane.Name == name {
			return lane
		}
	}
	t.Fatalf("lane %q not found", name)
	return IngestLane{}
}
