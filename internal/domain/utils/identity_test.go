package utils_test

import (
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestNormalizeListingURL_equivalence(t *testing.T) {
	want := "https://example.com/jobs/42"
	cases := []string{
		"https://example.com/jobs/42",
		"https://EXAMPLE.com/jobs/42",
		"https://example.com/jobs/42/",
		"https://example.com/jobs/42#section",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			got, err := utils.NormalizeListingURL(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

func TestStableJobID_sourceMatters(t *testing.T) {
	u := "https://example.com/jobs/1"
	a, err := utils.StableJobID("djinni", u)
	if err != nil {
		t.Fatal(err)
	}
	b, err := utils.StableJobID("linkedin", u)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("expected different ids for different sources")
	}
}

func TestStableJobID_errors(t *testing.T) {
	if _, err := utils.StableJobID("", "https://a.com/x"); err == nil {
		t.Fatal("want error for empty source")
	}
	if _, err := utils.StableJobID("x", ""); err == nil {
		t.Fatal("want error for empty URL")
	}
	if _, err := utils.StableJobID("x", "not-a-url"); err == nil {
		t.Fatal("want error for bad URL")
	}
}

func TestAssignStableID_fallbackApplyURL(t *testing.T) {
	j := &schema.Job{
		Source:   "board",
		URL:      "",
		ApplyURL: "https://apply.example.com/abc",
	}
	if err := utils.AssignStableID(j); err != nil {
		t.Fatal(err)
	}
	j2 := &schema.Job{Source: "board", URL: "", ApplyURL: "https://apply.example.com/abc"}
	if err := utils.AssignStableID(j2); err != nil {
		t.Fatal(err)
	}
	if j.ID != j2.ID {
		t.Fatalf("ids differ: %q vs %q", j.ID, j2.ID)
	}
}

func TestAssignStableID_prefersListingOverApply(t *testing.T) {
	j := &schema.Job{
		Source:   "board",
		URL:      "https://site.com/job/1",
		ApplyURL: "https://other.com/apply",
	}
	if err := utils.AssignStableID(j); err != nil {
		t.Fatal(err)
	}
	want, err := utils.StableJobID("board", "https://site.com/job/1")
	if err != nil {
		t.Fatal(err)
	}
	if j.ID != want {
		t.Fatalf("got %q want %q", j.ID, want)
	}
}
