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
	}, got.LocationCountries)
	require.Equal(t, []string{"WORLDWIDE", "EUROPE", "EMEA"}, got.LocationRegions)
	require.Len(t, got.ExcludeTitle, 155)
	require.Len(t, got.ExcludeText, 194)
	require.Len(t, got.Penalties, 5)
	require.Contains(t, got.ExcludeTitle, "recruiter")
	require.Contains(t, got.ExcludeTitle, "manager")
	require.Contains(t, got.ExcludeTitle, "junior")
	require.Contains(t, got.ExcludeTitle, "staff engineer")
	require.Contains(t, got.ExcludeTitle, "swift developer")
	require.Contains(t, got.ExcludeTitle, "java")
	require.Contains(t, got.ExcludeText, "php")
	require.Contains(t, got.ExcludeText, "react-native")
	require.Contains(t, got.ExcludeText, "pacific time")
	require.Contains(t, got.ExcludeText, "apac timezone only")
	require.Contains(t, got.ExcludeText, "americas overlap")
	for _, phrase := range []string{"uk only", "russia", "india", "argentina", "must be based in germany", "poland only", "canada only"} {
		require.NotContains(t, got.ExcludeText, phrase)
	}
	for _, phrase := range []string{"swift", "fraud", "wallet", "machine learning", "android", "soc 2", "java"} {
		require.NotContains(t, got.ExcludeText, phrase)
	}
	require.Contains(t, got.Penalties, "work permit required")
	require.Contains(t, got.Penalties, "right to work in germany")
	for _, phrase := range []string{
		"must reside in the us",
		"must be based in the us",
		"authorized to work in the us",
		"authorized to work in the united states",
		"authorized to work in the u.s",
		"authorization to work in the us",
		"us citizenship",
		"u.s. citizenship",
		"u.s. citizen",
		"us work authorization",
	} {
		require.Contains(t, got.ExcludeText, phrase)
		require.NotContains(t, got.ExcludeTitle, phrase)
		require.NotContains(t, got.Penalties, phrase)
	}
	for _, phrase := range []string{
		"united states only",
		"us based applicants only",
		"eastern time",
		"overlap with us",
	} {
		require.NotContains(t, got.ExcludeTitle, phrase)
		require.NotContains(t, got.ExcludeText, phrase)
		require.NotContains(t, got.Penalties, phrase)
	}

	listed, err := store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, []schema.Profile{got}, listed)
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
exclude_text: []
penalties: []
min_score: 1
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
exclude_title: []
exclude_text: []
penalties: []
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
exclude_title: []
exclude_text: []
penalties: []
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
exclude_title: [recruiter]
exclude_text: []
penalties: []
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
exclude_title: [recruiter]
exclude_text: []
penalties: []
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
