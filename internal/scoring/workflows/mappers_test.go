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

func TestIngestChildren_profileChildrenAndArchitectureLane(t *testing.T) {
	profile := profileschema.Profile{
		Sources: []string{"hipo", "landscape_institute", "ejobs", "olx_ro", "builtin"},
		Queries: []string{"arhitect", "peisagist"},
		Children: map[string][]string{
			"hipo":                {"Arhitectura-Design-Proiectare"},
			"landscape_institute": {"catalog"},
		},
	}

	got := IngestChildren(profile)
	want := []ingestschema.IngestSourceInput{
		{SourceID: "hipo", Query: "Arhitectura-Design-Proiectare"},
		{SourceID: "landscape_institute"},
		{SourceID: "ejobs", Query: "arhitect"},
		{SourceID: "ejobs", Query: "peisagist"},
		{SourceID: "olx_ro", Query: "arhitect"},
		{SourceID: "olx_ro", Query: "peisagist"},
		{SourceID: "builtin", Query: "arhitect"},
		{SourceID: "builtin", Query: "peisagist"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %#v, want %#v", got, want)
	}
	if got[0].Queries != nil {
		t.Fatalf("hipo copied queries %#v", got[0].Queries)
	}
	if got[1].Query != "" {
		t.Fatalf("catalog query = %q", got[1].Query)
	}

	lanes := IngestLanes(profile)
	arch := laneNamed(t, lanes, "architecture-browser")
	if arch.Name == "browser" {
		t.Fatal("architecture lane joined browser")
	}
	if !reflect.DeepEqual(arch.Children, want[2:6]) {
		t.Fatalf("architecture lane = %#v", arch.Children)
	}
	browser := laneNamed(t, lanes, "browser")
	if !reflect.DeepEqual(browser.Children, want[6:]) {
		t.Fatalf("browser lane = %#v", browser.Children)
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
