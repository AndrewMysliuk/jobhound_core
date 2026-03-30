package utils

import (
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

func TestValidateBroadFilterRules(t *testing.T) {
	t.Parallel()
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		rules   pipeline.BroadFilterRules
		wantErr bool
	}{
		{"both nil", pipeline.BroadFilterRules{}, false},
		{"both set ok", pipeline.BroadFilterRules{From: &from, To: &to}, false},
		{"only From", pipeline.BroadFilterRules{From: &from}, true},
		{"only To", pipeline.BroadFilterRules{To: &to}, true},
		{"From after To", pipeline.BroadFilterRules{From: &to, To: &from}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateBroadFilterRules(tc.rules)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestApplyBroadFilter_dateWindow_default7dUTC(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 3, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	winStart := now.Add(-7 * 24 * time.Hour)

	inside := domain.Job{
		ID: "a", Title: "x", Description: "y",
		PostedAt: winStart.Add(time.Hour),
	}
	tooOld := domain.Job{
		ID: "b", Title: "x", Description: "y",
		PostedAt: winStart.Add(-time.Second),
	}
	boundary := domain.Job{
		ID: "c", Title: "x", Description: "y",
		PostedAt: winStart,
	}
	atNow := domain.Job{
		ID: "d", Title: "x", Description: "y",
		PostedAt: now,
	}
	unknownDate := domain.Job{ID: "e", Title: "x", Description: "y"}

	got, err := ApplyBroadFilter(clock, pipeline.BroadFilterRules{}, []domain.Job{inside, tooOld, boundary, atNow, unknownDate})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d jobs, want 3: %+v", len(got), jobIDs(got))
	}
}

func TestApplyBroadFilter_dateWindow_explicit(t *testing.T) {
	t.Parallel()
	from := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 2, 28, 23, 59, 59, 0, time.UTC)
	clock := func() time.Time { return time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC) }

	in := domain.Job{
		ID: "ok", Title: "t", Description: "d",
		PostedAt: time.Date(2025, 2, 15, 10, 0, 0, 0, time.UTC),
	}
	before := domain.Job{
		ID: "old", Title: "t", Description: "d",
		PostedAt: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	rules := pipeline.BroadFilterRules{From: &from, To: &to}

	got, err := ApplyBroadFilter(clock, rules, []domain.Job{in, before})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "ok" {
		t.Fatalf("got %+v", got)
	}
}

func TestApplyBroadFilter_roleSynonyms(t *testing.T) {
	t.Parallel()
	posted := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return time.Date(2025, 3, 30, 0, 0, 0, 0, time.UTC) }
	winFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	winTo := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	rules := pipeline.BroadFilterRules{
		From:         &winFrom,
		To:           &winTo,
		RoleSynonyms: []string{"Frontend", "React"},
	}
	matchTitle := domain.Job{
		ID: "t1", Title: "Senior Frontend Developer", Description: "stuff",
		PostedAt: posted,
	}
	matchDesc := domain.Job{
		ID: "t2", Title: "Engineer", Description: "We use react daily",
		PostedAt: posted,
	}
	noMatch := domain.Job{
		ID: "t3", Title: "Backend", Description: "Go only",
		PostedAt: posted,
	}
	got, err := ApplyBroadFilter(clock, rules, []domain.Job{matchTitle, matchDesc, noMatch})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v", jobIDs(got))
	}
}

func TestApplyBroadFilter_remoteOnly(t *testing.T) {
	t.Parallel()
	posted := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return time.Date(2025, 3, 30, 0, 0, 0, 0, time.UTC) }
	winFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	winTo := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	rules := pipeline.BroadFilterRules{From: &winFrom, To: &winTo, RemoteOnly: true}

	remoteTrue := domain.Job{ID: "r1", Title: "x", Description: "y", PostedAt: posted, Remote: boolPtr(true)}
	remoteFalse := domain.Job{ID: "r2", Title: "x", Description: "y", PostedAt: posted, Remote: boolPtr(false)}
	remoteUnknown := domain.Job{ID: "r3", Title: "x", Description: "y", PostedAt: posted, Remote: nil}

	got, err := ApplyBroadFilter(clock, rules, []domain.Job{remoteTrue, remoteFalse, remoteUnknown})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "r1" {
		t.Fatalf("got %+v", got)
	}
}

func TestApplyBroadFilter_countryAllowlist(t *testing.T) {
	t.Parallel()
	posted := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return time.Date(2025, 3, 30, 0, 0, 0, 0, time.UTC) }
	winFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	winTo := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	rules := pipeline.BroadFilterRules{From: &winFrom, To: &winTo, CountryAllowlist: []string{"DE", "pl"}}

	de := domain.Job{ID: "c1", Title: "x", Description: "y", PostedAt: posted, CountryCode: "de"}
	plLower := domain.Job{ID: "c2", Title: "x", Description: "y", PostedAt: posted, CountryCode: "pl"}
	us := domain.Job{ID: "c3", Title: "x", Description: "y", PostedAt: posted, CountryCode: "US"}
	unknownCountry := domain.Job{ID: "c4", Title: "x", Description: "y", PostedAt: posted, CountryCode: ""}

	got, err := ApplyBroadFilter(clock, rules, []domain.Job{de, plLower, us, unknownCountry})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v", jobIDs(got))
	}
}

func TestApplyBroadFilter_emptyAllowlistNoCountryFilter(t *testing.T) {
	t.Parallel()
	posted := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return time.Date(2025, 3, 30, 0, 0, 0, 0, time.UTC) }
	winFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	winTo := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	rules := pipeline.BroadFilterRules{From: &winFrom, To: &winTo, CountryAllowlist: nil}

	unknownCountry := domain.Job{ID: "c4", Title: "x", Description: "y", PostedAt: posted, CountryCode: ""}
	got, err := ApplyBroadFilter(clock, rules, []domain.Job{unknownCountry})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatal("expected unknown country to pass when allowlist empty")
	}
}

func TestApplyBroadFilter_preservesOrder(t *testing.T) {
	t.Parallel()
	posted := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return time.Date(2025, 3, 30, 0, 0, 0, 0, time.UTC) }
	winFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	winTo := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	j1 := domain.Job{ID: "1", Title: "a", Description: "b", PostedAt: posted}
	j2 := domain.Job{ID: "2", Title: "a", Description: "b", PostedAt: posted}
	got, err := ApplyBroadFilter(clock, pipeline.BroadFilterRules{From: &winFrom, To: &winTo}, []domain.Job{j1, j2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "1" || got[1].ID != "2" {
		t.Fatalf("got %+v", jobIDs(got))
	}
}

func boolPtr(b bool) *bool { return &b }

func jobIDs(jobs []domain.Job) []string {
	out := make([]string, len(jobs))
	for i := range jobs {
		out[i] = jobs[i].ID
	}
	return out
}
