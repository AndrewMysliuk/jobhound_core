package impl

import (
	"testing"

	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
)

func TestNormalizeListStatusFilter_stage2Unknown(t *testing.T) {
	got, err := normalizeListStatusFilter(2, string(pipelineschema.RunJobUnknownStage2))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(pipelineschema.RunJobUnknownStage2) {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeListStatusFilter_stage2Eligible(t *testing.T) {
	got, err := normalizeListStatusFilter(2, pipelineschema.Stage2ListFilterEligible)
	if err != nil {
		t.Fatal(err)
	}
	if got != pipelineschema.Stage2ListFilterEligible {
		t.Fatalf("got %q", got)
	}
}
