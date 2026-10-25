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
	"dou_ua",
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
		"AT", "BE", "CY", "CZ", "DE", "DK", "EE", "ES", "FI", "FR", "GR", "HR", "HU", "IE", "IT",
		"LT", "LU", "LV", "MT", "NL", "PL", "PT", "RO", "SE", "SI", "SK",
		"IS", "LI", "NO", "CH", "GB",
	}, got.GateLocation.AllowCountries)
	require.Equal(t, []string{"WORLDWIDE", "EUROPE", "EMEA"}, got.GateLocation.AllowRegions)
	require.Contains(t, got.GateLocation.RejectTimezones, "pacific time")
	require.Contains(t, got.GateLocation.RejectTimezones, "pst")

	require.Equal(t, []string{"staff", "principal", "distinguished"}, got.GateRole)
	require.Contains(t, got.GateRoleAllow, "software engineer")
	require.Contains(t, got.GateRoleAllow, "vue developer")
	require.Contains(t, got.GateRoleAllow, "frontend engineer")
	require.Contains(t, got.GateRoleAllow, "webontwikkelaar")
	require.Contains(t, got.GateRoleAllow, "entwickler")
	require.Contains(t, got.GateRoleAllow, "ts dev")
	require.NotContains(t, got.GateRoleAllow, "platform engineer")
	require.NotContains(t, got.GateRoleAllow, "recruiter")
	require.NotContains(t, got.GateRoleAllow, "engineering manager")

	for _, phrase := range []string{
		"security clearance",
		"deferred compensation",
		"staff augmentation",
		"gambling",
		"crypto",
		"blockchain",
		"stablecoin",
	} {
		require.Contains(t, got.GateHard, phrase)
	}
	for _, phrase := range []string{
		"authorized to work in the us",
		"authorized to work in the united states",
		"us work authorization",
		"no visa sponsorship",
	} {
		require.NotContains(t, got.GateHard, phrase)
	}

	require.Equal(t, map[string]int{
		"vue": 6, "vuejs": 6, "typescript": 4, "react": 1, "golang": 3, "node": 4, "nodejs": 4,
	}, got.SignalsPlus.Title)
	require.Equal(t, 8, got.SignalsPlus.Text.MaxTotal)
	require.Equal(t, map[string]int{
		"vue": 3, "vuejs": 3, "typescript": 1, "golang": 1, "node": 1, "nodejs": 1,
	}, got.SignalsPlus.Text.Terms)
	require.NotContains(t, got.SignalsPlus.Text.Terms, "react")
	require.NotContains(t, got.SignalsPlus.Text.Terms, "max_total")
	require.Equal(t, 72000, got.SignalsPlus.Salary.MinEURYear)
	require.Equal(t, 120000, got.SignalsPlus.Salary.MinUSDYear)
	require.InDelta(t, 1.1, got.SignalsPlus.Salary.USDPerEUR, 0.0001)
	require.Equal(t, 4, got.SignalsPlus.Salary.BonusIfAbove)
	require.Equal(t, -4, got.SignalsPlus.Salary.PenaltyIfBelow)
	require.Equal(t, 2, got.SignalsPlus.Freshness.Days3)
	require.Equal(t, 1, got.SignalsPlus.Freshness.Days7)
	require.Equal(t, -2, got.SignalsPlus.Freshness.UnknownPostedAt)
	require.Equal(t, 8, got.Buckets.Top)
	require.Equal(t, []string{"EUROPE"}, got.Facts.ResidenceRegions)
	require.Equal(t, []string{"RO"}, got.Facts.ResidenceCountries)
	require.True(t, got.Facts.Office.Scored)
	require.Equal(t, got.GateLocation.AllowCountries, got.Facts.Office.AllowCountries)
	require.Equal(t, []string{"EUROPE", "EMEA", "WORLDWIDE"}, got.Facts.Office.AllowRegions)
	require.True(t, got.Facts.HiringScope.Scored)
	require.Equal(t, 5, got.Facts.HiringScope.Delta)
	require.Equal(t, -4, got.Facts.HiringScope.NarrowPenalty)
	require.True(t, got.Facts.WorkAuthorization.Scored)
	require.Equal(t, []string{"required"}, got.Facts.WorkAuthorization.Gates)
	require.True(t, got.Facts.Timezone.Scored)
	require.Equal(t, []string{"CET", "GMT", "EET"}, got.Facts.Timezone.Allow)
	require.False(t, got.Facts.Compensation.Scored)
	require.Equal(t, []string{"EUR", "USD", "GBP"}, got.Facts.Compensation.AllowCurrencies)
	require.True(t, got.Facts.WorkMode.Scored)
	require.Equal(t, []string{"hybrid", "office"}, got.Facts.WorkMode.Gates)
	require.False(t, got.Facts.EmploymentLoad.Scored)
	require.False(t, got.Facts.Engagement.Scored)

	names := make([]string, 0, len(got.PenaltiesSoft))
	byName := make(map[string]schema.PenaltyGroup, len(got.PenaltiesSoft))
	for _, group := range got.PenaltiesSoft {
		names = append(names, group.Name)
		byName[group.Name] = group
	}
	require.Equal(t, []string{
		"agency_smell",
		"culture_smell",
		"foreign_domain",
		"foreign_stack",
		"ml_domain",
		"office_pressure",
		"part_time",
		"react_primary",
		"security_domain",
	}, names)
	require.Equal(t, -7, byName["agency_smell"].Weight)
	require.Contains(t, byName["agency_smell"].Terms, "for our client")
	require.Contains(t, byName["agency_smell"].Terms, "recruitment agency")
	require.Equal(t, 2, byName["foreign_stack"].MinOccurrences)
	require.Equal(t, -4, byName["foreign_stack"].Weight)
	require.False(t, byName["foreign_stack"].CheckNegation)
	require.Contains(t, byName["foreign_stack"].Terms, "php")
	require.Contains(t, byName["foreign_stack"].Terms, "react-native")
	require.Contains(t, byName["foreign_stack"].Terms, "java")
	require.Equal(t, 2, byName["react_primary"].MinOccurrences)
	require.Equal(t, -6, byName["react_primary"].Weight)
	require.Contains(t, byName["react_primary"].Terms, "react")
	require.Contains(t, byName["react_primary"].Terms, "next.js")
	require.Contains(t, byName["office_pressure"].Terms, "hybrid role")
	require.NotContains(t, byName["office_pressure"].Terms, "work permit required")
	require.NotContains(t, byName["office_pressure"].Terms, "right to work in germany")
	require.NotContains(t, byName["office_pressure"].Terms, "must be based in")
	require.True(t, byName["part_time"].CheckNegation)
	require.Contains(t, byName["part_time"].Terms, "part-time")
	for _, name := range names {
		if name == "foreign_stack" || name == "react_primary" {
			continue
		}
		require.Equal(t, 1, byName[name].MinOccurrences)
		require.Negative(t, byName[name].Weight)
	}
	for _, name := range []string{"agency_smell", "culture_smell", "foreign_domain", "foreign_stack", "ml_domain", "office_pressure", "react_primary", "security_domain"} {
		require.False(t, byName[name].CheckNegation)
	}

	for _, phrase := range []string{"uk only", "russia", "india", "argentina", "must be based in germany", "poland only", "canada only"} {
		require.NotContains(t, byName["office_pressure"].Terms, phrase)
	}
	for _, phrase := range []string{"swift", "fraud", "wallet", "machine learning", "android", "soc 2"} {
		require.NotContains(t, byName["foreign_stack"].Terms, phrase)
		require.NotContains(t, byName["ml_domain"].Terms, phrase)
	}
	for _, phrase := range []string{
		"united states only",
		"us based applicants only",
		"eastern time",
		"overlap with us",
	} {
		require.NotContains(t, got.GateRoleAllow, phrase)
		require.NotContains(t, got.GateHard, phrase)
		require.NotContains(t, got.GateLocation.RejectTimezones, phrase)
	}

	listed, err := store.List(context.Background())
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, got, listed[0])

	lada, err := store.Get(context.Background(), "lada")
	require.NoError(t, err)
	require.Equal(t, listed[1], lada)
	require.Equal(t, "Lada", lada.Name)
	require.Equal(t, schema.DomainArchitecture, lada.Domain)
	require.Equal(t, []string{
		"bestjobs",
		"hipo",
		"ejobs",
		"dezeen_jobs",
		"riba_jobs",
		"landscape_institute",
		"archgee",
		"archinect",
		"competitionline",
		"baunetz",
		"bdla",
		"world_architects",
	}, lada.Sources)
	for _, id := range lada.Sources {
		require.Contains(t, known, id)
	}
	require.Equal(t, []string{
		"arhitect proiectant",
		"arhitect brasov",
		"peisagist",
		"peisagist brasov",
		"urbanist",
	}, lada.Queries)
	require.Equal(t, map[string][]string{
		"hipo":                {"Arhitectura-Design-Proiectare"},
		"dezeen_jobs":         {"architectural-designer", "architectural-assistants", "landscape-architect", "part-1", "part-2"},
		"riba_jobs":           {"architectural-assistant", "architect", "part-1", "part-2", "landscape-and-public-realm"},
		"landscape_institute": {"catalog"},
		"archgee":             {"architect", "landscape-architect", "remote"},
		"archinect":           {"remote"},
		"competitionline":     {"catalog"},
		"baunetz":             {"catalog"},
		"bdla":                {"catalog"},
		"world_architects":    {"catalog"},
	}, lada.Children)
	require.NotContains(t, lada.Children, "bestjobs")
	require.NotContains(t, lada.Children, "ejobs")
	require.Equal(t, map[string]int{
		"architectural designer":     6,
		"architectural assistant":    6,
		"landscape designer":         6,
		"arhitect proiectant":        6,
		"arhitect junior":            6,
		"peisagist":                  6,
		"landscape architect":        5,
		"architectural technologist": 4,
		"graduate architect":         4,
		"junior architect":           4,
		"arhitect":                   4,
		"urbanist":                   3,
		"desenator tehnic":           3,
	}, lada.SignalsPlus.Title)
	require.Equal(t, 4, lada.SignalsPlus.Text.MaxTotal)
	require.Equal(t, 6, lada.Buckets.Top)
	require.NotContains(t, lada.GateLocation.AllowCountries, "US")
	require.Equal(t, []string{"EUROPE"}, lada.Facts.ResidenceRegions)
	require.True(t, lada.Facts.Office.Scored)
	require.Equal(t, lada.GateLocation.AllowCountries, lada.Facts.Office.AllowCountries)
	require.NotContains(t, lada.Facts.Office.AllowCountries, "US")
	require.Equal(t, []string{"EUROPE", "EMEA", "WORLDWIDE"}, lada.Facts.Office.AllowRegions)
	require.True(t, lada.Facts.HiringScope.Scored)
	require.True(t, lada.Facts.WorkAuthorization.Scored)
	require.Equal(t, []string{"required"}, lada.Facts.WorkAuthorization.Gates)
	require.True(t, lada.Facts.Timezone.Scored)
	require.False(t, lada.Facts.Compensation.Scored)
	require.False(t, lada.Facts.WorkMode.Scored)
	require.False(t, lada.Facts.EmploymentLoad.Scored)
	require.False(t, lada.Facts.Engagement.Scored)
	require.Empty(t, lada.WellfoundRoles)
	require.Empty(t, lada.GateLocation.RejectTimezones)
	require.NotContains(t, lada.GateRole, "intern")
	require.NotContains(t, lada.GateRole, "internship")
	require.Contains(t, lada.GateRole, "interior designer")
	require.Contains(t, lada.GateRole, "solution architect")
	require.NotContains(t, lada.GateRole, "technical architect")
	require.NotContains(t, lada.GateHard, "right to work in the uk")
	require.Contains(t, lada.GateHard, "drept de semnatura obligatoriu")
	require.Contains(t, lada.GateHard, "deutschkenntnisse")
	require.Contains(t, lada.GateHard, "verhandlungssicheres deutsch")
	require.Contains(t, lada.GateHard, "fließende deutschkenntnisse")
	require.NotContains(t, lada.GateHard, "drept de semnatura")
	require.NotContains(t, lada.GateHard, "work permit required")
	require.NotContains(t, lada.GateHard, "itar")
	require.NotContains(t, lada.GateHard, "govcloud")
	ladaPenalties := make([]string, 0, len(lada.PenaltiesSoft))
	for _, group := range lada.PenaltiesSoft {
		ladaPenalties = append(ladaPenalties, group.Name)
	}
	require.Equal(t, []string{"agency", "culture_smell", "mid_level", "not_active", "part_time", "wrong_domain"}, ladaPenalties)
	for _, phrase := range []string{"designer", "architect", "assistant", "landscape", "junior"} {
		require.NotContains(t, lada.GateRole, phrase)
	}
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
	require.Empty(t, got.Facts.ResidenceRegions)
	require.False(t, got.Facts.Office.Scored)
	require.False(t, got.Facts.HiringScope.Scored)
	require.False(t, got.Facts.WorkAuthorization.Scored)
	require.False(t, got.Facts.Timezone.Scored)
	require.False(t, got.Facts.Compensation.Scored)
	require.False(t, got.Facts.WorkMode.Scored)
	require.False(t, got.Facts.EmploymentLoad.Scored)
	require.False(t, got.Facts.Engagement.Scored)
	require.Zero(t, got.Facts.Office.Delta)
	require.Zero(t, got.Facts.HiringScope.Delta)
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

func TestFileStoreRoundTripsChildren(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeProfile(t, dir, "sample", `
id: sample
name: Sample
domain: software
sources: [vue_jobs]
queries: []
wellfound_roles: []
children:
  vue_jobs:
    - catalog
`)
	got, err := NewFileStore(dir, sourceids.KnownSourceIDs()).Get(context.Background(), "sample")
	require.NoError(t, err)
	require.Equal(t, map[string][]string{"vue_jobs": {"catalog"}}, got.Children)
}

func TestFileStoreRejectsChildrenSourceNotInSources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeProfile(t, dir, "sample", `
id: sample
name: Sample
domain: software
sources: [vue_jobs]
queries: []
wellfound_roles: []
children:
  himalayas:
    - catalog
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

func TestFileStoreAcceptsDouUA(t *testing.T) {
	t.Parallel()
	require.Contains(t, sourceids.KnownSourceIDs(), "dou_ua")
	dir := t.TempDir()
	writeProfile(t, dir, "sample", `
id: sample
name: Sample
domain: software
sources: [dou_ua]
queries: [golang]
wellfound_roles: []
`)
	got, err := NewFileStore(dir, sourceids.KnownSourceIDs()).Get(context.Background(), "sample")
	require.NoError(t, err)
	require.Equal(t, []string{"dou_ua"}, got.Sources)
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
