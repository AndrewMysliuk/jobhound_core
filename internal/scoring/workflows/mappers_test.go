package scoring_workflows

import (
	"reflect"
	"testing"

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

func TestIngestChildren_profileChildrenAndPerSourceLanes(t *testing.T) {
	profile := profileschema.Profile{
		Sources: []string{"hipo", "landscape_institute", "ejobs", "dezeen_jobs", "world_architects", "builtin", "golang_cafe"},
		Queries: []string{"arhitect", "peisagist"},
		Children: map[string][]string{
			"hipo":                {"Arhitectura-Design-Proiectare"},
			"landscape_institute": {"catalog"},
			"dezeen_jobs":         {"architectural-designer"},
			"world_architects":    {"catalog"},
		},
	}

	got := IngestChildren(profile)
	want := []ingestschema.IngestSourceInput{
		{SourceID: "hipo", Query: "Arhitectura-Design-Proiectare"},
		{SourceID: "landscape_institute"},
		{SourceID: "ejobs", Query: "arhitect"},
		{SourceID: "ejobs", Query: "peisagist"},
		{SourceID: "dezeen_jobs", Query: "architectural-designer"},
		{SourceID: "world_architects"},
		{SourceID: "builtin", Query: "arhitect"},
		{SourceID: "builtin", Query: "peisagist"},
		{SourceID: "golang_cafe"},
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
	ejobs := laneNamed(t, lanes, "ejobs")
	if !reflect.DeepEqual(ejobs.Children, want[2:4]) {
		t.Fatalf("ejobs lane = %#v", ejobs.Children)
	}
	builtin := laneNamed(t, lanes, "builtin")
	if !reflect.DeepEqual(builtin.Children, want[6:8]) {
		t.Fatalf("builtin lane = %#v", builtin.Children)
	}
	cafe := laneNamed(t, lanes, "golang_cafe")
	if !reflect.DeepEqual(cafe.Children, want[8:]) {
		t.Fatalf("golang_cafe lane = %#v", cafe.Children)
	}
	if len(lanes) != 7 {
		t.Fatalf("lanes = %d, want 7 (one per source)", len(lanes))
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
