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

func TestStableJobID_collapsesTitleWhitespace(t *testing.T) {
	got := utils.StableJobID("acme", "  Senior   Go ")
	want := "acme\x1esenior go"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAssignStableID_ignoresLocationAndApplyURL(t *testing.T) {
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
		Location: schema.Location{Raw: "United States"},
	}
	if err := utils.AssignStableID(a); err != nil {
		t.Fatal(err)
	}
	if err := utils.AssignStableID(b); err != nil {
		t.Fatal(err)
	}
	want := "acme\x1eengineer"
	if a.ID != want || b.ID != want {
		t.Fatalf("ids %q %q want %q", a.ID, b.ID, want)
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

func TestAssignStableID_placeholderIncludesListingURL(t *testing.T) {
	cases := []struct {
		company, wantKey string
	}{
		{company: "", wantKey: ""},
		{company: "Confidential", wantKey: "confidential"},
		{company: "Confidențial", wantKey: "confidențial"},
		{company: "Nedivulgat", wantKey: "nedivulgat"},
	}
	for _, tc := range cases {
		t.Run(tc.wantKey, func(t *testing.T) {
			j := &schema.Job{
				Company: tc.company,
				Title:   "  Architect ",
				URL:     "https://EXAMPLE.com/jobs/42/?ref=1",
			}
			if err := utils.AssignStableID(j); err != nil {
				t.Fatal(err)
			}
			want := tc.wantKey + "\x1earchitect\x1ehttps://example.com/jobs/42"
			if j.ID != want {
				t.Fatalf("id %q want %q", j.ID, want)
			}
			if j.CompanyKey != tc.wantKey {
				t.Fatalf("key %q want %q", j.CompanyKey, tc.wantKey)
			}
		})
	}
}

func TestAssignStableID_namedCompanyOmitsListingURL(t *testing.T) {
	j := &schema.Job{
		Company: "Acme Inc",
		Title:   "Architect",
		URL:     "https://example.com/jobs/42",
	}
	if err := utils.AssignStableID(j); err != nil {
		t.Fatal(err)
	}
	want := "acme\x1earchitect"
	if j.ID != want {
		t.Fatalf("id %q want %q", j.ID, want)
	}
}
