package impl

import (
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

func TestNormalizeListStatusFilter_stage2Unknown(t *testing.T) {
	got, err := normalizeListStatusFilter(2, string(pipeline.RunJobUnknownStage2))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(pipeline.RunJobUnknownStage2) {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeListStatusFilter_stage2Eligible(t *testing.T) {
	got, err := normalizeListStatusFilter(2, pipeline.Stage2ListFilterEligible)
	if err != nil {
		t.Fatal(err)
	}
	if got != pipeline.Stage2ListFilterEligible {
		t.Fatalf("got %q", got)
	}
}
