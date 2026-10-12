package impl

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/sourceids"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
)

var contractSourceIDs = []string{
	"working_nomads",
	"himalayas",
	"remotify_europe",
	"we_work_remotely",
	"wellfound",
	"vue_jobs",
	"golang_cafe",
	"builtin",
	"consider_andreessen_horowitz",
	"consider_lightspeed",
	"consider_sequoia_capital",
	"consider_gv",
	"consider_bessemer_ventures",
	"consider_balderton_capital",
	"consider_notion_capital",
	"consider_atlantic_labs",
	"consider_battery_ventures",
	"consider_panoramic_ventures",
	"consider_ivp",
	"consider_kleiner_perkins",
	"consider_nea",
	"consider_sequoia_capital_india",
	"consider_union_square_ventures",
	"getro_accel",
	"getro_general_catalyst",
	"getro_atomico",
	"getro_hv_capital",
	"getro_point_nine",
	"getro_earlybird",
	"getro_firstminute",
	"getro_craft_ventures",
}

func TestFileStoreLoadsAndrew(t *testing.T) {
	t.Parallel()
	dir := profilesDir(t)
	_, err := os.Stat(filepath.Join(dir, "architecture.yaml"))
	if !os.IsNotExist(err) {
		t.Fatalf("architecture.yaml present: %v", err)
	}

	store := NewFileStore(dir, sourceids.KnownSourceIDs())
	got, err := store.Get(context.Background(), "andrew")
	require.NoError(t, err)
	require.Equal(t, "andrew", got.ID)
	require.Equal(t, "Andrew", got.Name)
	require.Equal(t, schema.DomainSoftware, got.Domain)
	require.Equal(t, contractSourceIDs, got.Sources)
	known := sourceids.KnownSourceIDs()
	for _, id := range contractSourceIDs {
		require.Contains(t, known, id)
	}
	require.Contains(t, known, "europe_remotely")
	require.Equal(t, []string{"vue", "typescript", "react", "golang", "node"}, got.Queries)
	require.Equal(t, []string{
		"frontend-engineer",
		"full-stack-engineer",
		"backend-engineer",
	}, got.WellfoundRoles)
	require.NotContains(t, got.Queries, "frontend")
	require.NotContains(t, got.Queries, "full-stack")
	require.Equal(t, []string{
		"US",
		"AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "ES", "FI", "FR", "GR", "HR", "HU", "IE", "IT",
		"LT", "LU", "LV", "MT", "NL", "PL", "PT", "RO", "SE", "SI", "SK",
		"IS", "LI", "NO", "CH", "GB",
	}, got.GateLocation.AllowCountries)
	require.Equal(t, []string{"WORLDWIDE", "EUROPE", "EMEA"}, got.GateLocation.AllowRegions)
	for _, phrase := range []string{"pacific time", "apac timezone only", "americas overlap"} {
		require.Contains(t, got.GateLocation.RejectTimezones, phrase)
	}
	for _, phrase := range got.GateLocation.RejectTimezones {
		require.NotContains(t, phrase, "utc")
	}

	require.Contains(t, got.GateRole, "recruiter")
	require.Contains(t, got.GateRole, "manager")
	require.Contains(t, got.GateRole, "junior")
	require.Contains(t, got.GateRole, "swift developer")
	require.Contains(t, got.GateRole, "java")
	require.Contains(t, got.GateRole, "developer productivity")
	for _, phrase := range []string{
		"staff engineer",
		"principal engineer",
		"lead engineer",
		"architect",
		"platform engineer",
		"ai platform engineer",
		"developer experience",
		"forward deployed",
	} {
		require.NotContains(t, got.GateRole, phrase)
	}

	for _, phrase := range []string{
		"authorized to work in the us",
		"authorized to work in the united states",
		"us work authorization",
		"security clearance",
		"no visa sponsorship",
		"deferred compensation",
	} {
		require.Contains(t, got.GateHard, phrase)
		require.NotContains(t, got.GateRole, phrase)
	}

	require.Equal(t, map[string]int{
		"vue": 6, "typescript": 4, "react": 4, "golang": 6, "node": 4,
	}, got.SignalsPlus.Title)
	require.Equal(t, 4, got.SignalsPlus.Text.MaxTotal)
	require.Equal(t, map[string]int{
		"vue": 2, "typescript": 2, "react": 2, "golang": 2, "node": 2,
	}, got.SignalsPlus.Text.Terms)
	require.NotContains(t, got.SignalsPlus.Text.Terms, "max_total")
	require.Equal(t, 80000, got.SignalsPlus.Salary.MinEURYear)
	require.Equal(t, 120000, got.SignalsPlus.Salary.MinUSDYear)
	require.InDelta(t, 1.1, got.SignalsPlus.Salary.USDPerEUR, 0.0001)
	require.Equal(t, 4, got.SignalsPlus.Salary.BonusIfAbove)
	require.Equal(t, -4, got.SignalsPlus.Salary.PenaltyIfBelow)
	require.Equal(t, 2, got.SignalsPlus.Freshness.Days3)
	require.Equal(t, 1, got.SignalsPlus.Freshness.Days7)
	require.Equal(t, 6, got.Buckets.Top)

	names := make([]string, 0, len(got.PenaltiesSoft))
	byName := make(map[string]schema.PenaltyGroup, len(got.PenaltiesSoft))
	for _, group := range got.PenaltiesSoft {
		names = append(names, group.Name)
		byName[group.Name] = group
	}
	require.Equal(t, []string{
		"agency",
		"ambiguous_us",
		"culture_smell",
		"foreign_domain",
		"foreign_stack",
		"gambling_crypto",
		"ml_domain",
		"office_pressure",
		"part_time",
		"security_domain",
	}, names)
	require.Equal(t, 2, byName["foreign_stack"].MinOccurrences)
	require.Equal(t, -4, byName["foreign_stack"].Weight)
	require.False(t, byName["foreign_stack"].CheckNegation)
	require.Contains(t, byName["foreign_stack"].Terms, "php")
	require.Contains(t, byName["foreign_stack"].Terms, "react-native")
	require.NotContains(t, byName["foreign_stack"].Terms, "java")
	require.Contains(t, byName["office_pressure"].Terms, "work permit required")
	require.Contains(t, byName["office_pressure"].Terms, "right to work in germany")
	require.True(t, byName["ambiguous_us"].CheckNegation)
	require.True(t, byName["part_time"].CheckNegation)
	require.Contains(t, byName["part_time"].Terms, "part-time")
	require.Contains(t, byName["ambiguous_us"].Terms, "must reside in the us")
	for _, name := range names {
		if name == "foreign_stack" {
			continue
		}
		require.Equal(t, 1, byName[name].MinOccurrences)
		require.Negative(t, byName[name].Weight)
	}
	for _, name := range []string{"agency", "culture_smell", "foreign_domain", "foreign_stack", "gambling_crypto", "ml_domain", "office_pressure", "security_domain"} {
		require.False(t, byName[name].CheckNegation)
	}

	for _, phrase := range []string{"uk only", "russia", "india", "argentina", "must be based in germany", "poland only", "canada only"} {
		require.NotContains(t, byName["ambiguous_us"].Terms, phrase)
		require.NotContains(t, byName["office_pressure"].Terms, phrase)
	}
	for _, phrase := range []string{"swift", "fraud", "wallet", "machine learning", "android", "soc 2", "java"} {
		require.NotContains(t, byName["foreign_stack"].Terms, phrase)
		require.NotContains(t, byName["ml_domain"].Terms, phrase)
	}
	for _, phrase := range []string{
		"united states only",
		"us based applicants only",
		"eastern time",
		"overlap with us",
	} {
		require.NotContains(t, got.GateRole, phrase)
		require.NotContains(t, got.GateHard, phrase)
		require.NotContains(t, got.GateLocation.RejectTimezones, phrase)
	}

	listed, err := store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, []schema.Profile{got}, listed)
}

func TestFileStoreDecodesFiveBlocks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeProfile(t, dir, "sample", `
id: sample
name: Sample
domain: software
sources: [vue_jobs]
queries: [vue]
wellfound_roles: []
gate_role: [recruiter]
gate_hard: [clearance]
gate_location:
  allow_countries: [DE]
  allow_regions: [EUROPE]
  reject_timezones: ["pacific time"]
signals_plus:
  title:
    vue: 6
  text:
    max_total: 4
    terms:
      vue: 2
  salary:
    min_eur_year: 80000
    min_usd_year: 120000
    usd_per_eur: 1.25
    bonus_if_above: 4
    penalty_if_below: -4
  freshness:
    days_3: 2
    days_7: 1
penalties_soft:
  zebra:
    weight: -1
    terms: [zzz]
    min_occurrences: 0
    check_negation: false
  alpha:
    weight: -2
    terms: [aaa]
    min_occurrences: 1
    check_negation: true
buckets:
  top: 6
`)
	got, err := NewFileStore(dir, sourceids.KnownSourceIDs()).Get(context.Background(), "sample")
	require.NoError(t, err)
	require.Equal(t, []string{"recruiter"}, got.GateRole)
	require.Equal(t, []string{"clearance"}, got.GateHard)
	require.Equal(t, []string{"DE"}, got.GateLocation.AllowCountries)
	require.Equal(t, []string{"EUROPE"}, got.GateLocation.AllowRegions)
	require.Equal(t, []string{"pacific time"}, got.GateLocation.RejectTimezones)
	require.Equal(t, map[string]int{"vue": 6}, got.SignalsPlus.Title)
	require.Equal(t, 4, got.SignalsPlus.Text.MaxTotal)
	require.Equal(t, map[string]int{"vue": 2}, got.SignalsPlus.Text.Terms)
	require.Equal(t, 1.25, got.SignalsPlus.Salary.USDPerEUR)
	require.Equal(t, schema.FreshnessSignals{Days3: 2, Days7: 1}, got.SignalsPlus.Freshness)
	require.Equal(t, []schema.PenaltyGroup{
		{Name: "alpha", Weight: -2, MinOccurrences: 1, CheckNegation: true, Terms: []string{"aaa"}},
		{Name: "zebra", Weight: -1, MinOccurrences: 0, CheckNegation: false, Terms: []string{"zzz"}},
	}, got.PenaltiesSoft)
	require.Equal(t, 6, got.Buckets.Top)
}

func TestFileStoreRejectsUnknownKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeProfile(t, dir, "sample", `
id: sample
name: Sample
domain: software
sources: [vue_jobs]
queries: []
wellfound_roles: []
exclude_title: []
`)
	_, err := NewFileStore(dir, sourceids.KnownSourceIDs()).Get(context.Background(), "sample")
	require.ErrorIs(t, err, profiles.ErrProfileInvalid)
}

func TestFileStoreRejectsUnknownSourceID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeProfile(t, dir, "sample", `
id: sample
name: Sample
domain: software
sources: [not_a_collector]
queries: []
wellfound_roles: []
`)
	_, err := NewFileStore(dir, sourceids.KnownSourceIDs()).Get(context.Background(), "sample")
	require.ErrorIs(t, err, profiles.ErrProfileInvalid)
}

func TestFileStoreAcceptsEuropeRemotely(t *testing.T) {
	t.Parallel()
	require.Contains(t, sourceids.KnownSourceIDs(), "europe_remotely")
	dir := t.TempDir()
	writeProfile(t, dir, "sample", `
id: sample
name: Sample
domain: software
sources: [europe_remotely]
queries: []
wellfound_roles: []
`)
	got, err := NewFileStore(dir, sourceids.KnownSourceIDs()).Get(context.Background(), "sample")
	require.NoError(t, err)
	require.Equal(t, []string{"europe_remotely"}, got.Sources)
}

func TestFileStoreGetReadsFileAgain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := NewFileStore(dir, sourceids.KnownSourceIDs())
	writeProfile(t, dir, "sample", `
id: sample
name: Alpha
domain: software
sources: [vue_jobs]
queries: [vue]
wellfound_roles: []
`)
	first, err := store.Get(context.Background(), "sample")
	require.NoError(t, err)
	require.Equal(t, "Alpha", first.Name)
	require.Equal(t, []string{"vue"}, first.Queries)

	writeProfile(t, dir, "sample", `
id: sample
name: Beta
domain: software
sources: [vue_jobs]
queries: [react]
wellfound_roles: []
`)
	second, err := store.Get(context.Background(), "sample")
	require.NoError(t, err)
	require.Equal(t, "Beta", second.Name)
	require.Equal(t, []string{"react"}, second.Queries)
}

func TestFileStoreUnknownID(t *testing.T) {
	t.Parallel()
	_, err := NewFileStore(t.TempDir(), sourceids.KnownSourceIDs()).Get(context.Background(), "missing")
	require.ErrorIs(t, err, profiles.ErrProfileNotFound)
}

func profilesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "config", "profiles")
}

func writeProfile(t *testing.T, dir, id, body string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, id+".yaml"), []byte(body), 0o644)
	require.NoError(t, err)
}
