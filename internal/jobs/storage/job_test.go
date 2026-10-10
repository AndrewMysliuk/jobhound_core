package storage

import (
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

func TestJobModel_roundTrip(t *testing.T) {
	posted := time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC)
	seen := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	in := schema.Job{
		ID:             "j1",
		Source:         "himalayas",
		Title:          "Eng",
		Company:        "Acme Inc",
		CompanyKey:     "acme",
		CompanyWebsite: "https://acme.example",
		URL:            "https://job",
		ApplyURL:       "https://ats",
		Description:    "text",
		PostedAt:       posted,
		Location: schema.Location{
			Type:      schema.LocationRemote,
			Regions:   []string{"EUROPE"},
			Countries: []string{"DE"},
			Timezone:  "5.5",
			Raw:       "Berlin",
		},
		SalaryRaw:   "€80k",
		Tags:        []string{"go"},
		Position:    strPtr("backend"),
		FirstSeenAt: seen,
		LastSeenAt:  seen,
	}
	m := NewJobModel(in)
	got := m.ToDomain()
	if got.Source != "" {
		t.Fatalf("Source is not stored, got %q", got.Source)
	}
	if len(got.Sources) != 1 || got.Sources[0] != "himalayas" {
		t.Fatalf("sources %#v", got.Sources)
	}
	got.Sources = nil
	in.Source = ""
	in.Sources = nil
	if !domainJobEqual(got, in) {
		t.Fatalf("round-trip got %+v want %+v", got, in)
	}
}

func TestJob_ToDomain_locationJSON(t *testing.T) {
	m := Job{
		Location: []byte(`{"type":"remote","regions":["EUROPE"],"countries":["DE"],"timezone":"","raw":"Berlin"}`),
		Sources:  textArray{"builtin", "himalayas"},
	}
	got := m.ToDomain()
	if got.Location.Type != schema.LocationRemote || got.Location.Raw != "Berlin" {
		t.Fatalf("location %+v", got.Location)
	}
	if len(got.Location.Countries) != 1 || got.Location.Countries[0] != "DE" {
		t.Fatalf("countries %#v", got.Location.Countries)
	}
	if len(got.Sources) != 2 || got.Sources[0] != "builtin" || got.Sources[1] != "himalayas" {
		t.Fatalf("sources %#v", got.Sources)
	}
}

func domainJobEqual(a, b schema.Job) bool {
	if a.ID != b.ID || a.Title != b.Title || a.Company != b.Company || a.CompanyKey != b.CompanyKey ||
		a.CompanyWebsite != b.CompanyWebsite || a.URL != b.URL || a.ApplyURL != b.ApplyURL ||
		a.Description != b.Description || a.SalaryRaw != b.SalaryRaw ||
		a.Location.Type != b.Location.Type || a.Location.Timezone != b.Location.Timezone ||
		a.Location.Raw != b.Location.Raw {
		return false
	}
	if !stringSliceEqual(a.Sources, b.Sources) || !stringSliceEqual(a.Tags, b.Tags) ||
		!stringSliceEqual(a.Location.Regions, b.Location.Regions) ||
		!stringSliceEqual(a.Location.Countries, b.Location.Countries) {
		return false
	}
	if !strPtrEqual(a.Position, b.Position) || !a.PostedAt.Equal(b.PostedAt) ||
		!a.FirstSeenAt.Equal(b.FirstSeenAt) || !a.LastSeenAt.Equal(b.LastSeenAt) {
		return false
	}
	return true
}

func strPtr(s string) *string { return &s }

func strPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func stringSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
