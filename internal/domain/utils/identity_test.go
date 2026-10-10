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
		"https://example.com/jobs/42?ref=1",
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

func TestCompanyKey_stripsLegalSuffixes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{in: "Acme Inc", want: "acme"},
		{in: "Acme, Inc.", want: "acme"},
		{in: "Foo GmbH", want: "foo"},
		{in: "Bar Ltd", want: "bar"},
		{in: "Baz LLC", want: "baz"},
		{in: "Quux SRL", want: "quux"},
		{in: "  Acme   Inc  ", want: "acme"},
		{in: "Included Labs", want: "included labs"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := utils.CompanyKey(tc.in); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestStableJobID_normalizesTitleAndRaw(t *testing.T) {
	got := utils.StableJobID("acme", "  Senior   Go ", " Berlin  DE ")
	want := "acme\x1esenior go\x1eberlin de"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	emptyRaw := utils.StableJobID("acme", "Senior Go", "")
	if emptyRaw != "acme\x1esenior go\x1e" {
		t.Fatalf("empty raw: got %q", emptyRaw)
	}
}

func TestAssignStableID_ignoresApplyURL(t *testing.T) {
	a := &schema.Job{
		Company:  "Acme Inc",
		Title:    "Engineer",
		ApplyURL: "https://boards.greenhouse.io/acme/jobs/1",
		Location: schema.Location{Raw: "Berlin"},
	}
	b := &schema.Job{
		Company:  "acme llc",
		Title:    "  engineer ",
		ApplyURL: "https://other.example/apply",
		Location: schema.Location{Raw: " berlin "},
	}
	if err := utils.AssignStableID(a); err != nil {
		t.Fatal(err)
	}
	if err := utils.AssignStableID(b); err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Fatalf("ids differ: %q vs %q", a.ID, b.ID)
	}
	if a.CompanyKey != "acme" || b.CompanyKey != "acme" {
		t.Fatalf("keys %q %q", a.CompanyKey, b.CompanyKey)
	}
}

func TestAssignStableID_nil(t *testing.T) {
	if err := utils.AssignStableID(nil); err == nil {
		t.Fatal("want error")
	}
}
