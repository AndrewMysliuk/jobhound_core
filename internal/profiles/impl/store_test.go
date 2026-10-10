package impl

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
)

var contractSourceIDs = []string{
	"europe_remotely",
	"working_nomads",
	"himalayas",
	"remotify_europe",
	"we_work_remotely",
	"wellfound",
	"vue_jobs",
	"golang_cafe",
	"builtin",
}

func TestFileStoreLoadsAndrew(t *testing.T) {
	t.Parallel()
	dir := profilesDir(t)
	_, err := os.Stat(filepath.Join(dir, "architecture.yaml"))
	if !os.IsNotExist(err) {
		t.Fatalf("architecture.yaml present: %v", err)
	}

	store := NewFileStore(dir, contractSourceIDs)
	got, err := store.Get(context.Background(), "andrew")
	require.NoError(t, err)
	require.Equal(t, "andrew", got.ID)
	require.Equal(t, "Andrew", got.Name)
	require.Equal(t, schema.DomainSoftware, got.Domain)
	require.Equal(t, contractSourceIDs, got.Sources)
	require.Equal(t, []string{"vue", "typescript", "react", "golang", "node", "AI native", "AI engineer", "LLM"}, got.Queries)
	require.Equal(t, []string{
		"frontend-engineer",
		"full-stack-engineer",
		"backend-engineer",
		"artificial-intelligence-engineer",
	}, got.WellfoundRoles)
	require.NotContains(t, got.Queries, "frontend")
	require.NotContains(t, got.Queries, "full-stack")
	require.Len(t, got.ExcludeTitle, 188)
	require.Len(t, got.ExcludeText, 167)
	require.Len(t, got.Penalties, 11)
	require.Contains(t, got.ExcludeTitle, "recruiter")
	require.Contains(t, got.ExcludeTitle, "product manager")
	require.Contains(t, got.ExcludeTitle, "junior")
	require.Contains(t, got.ExcludeTitle, "staff engineer")
	require.Contains(t, got.ExcludeText, "php")
	require.Contains(t, got.ExcludeText, "java")
	require.Contains(t, got.ExcludeText, "uk only")
	require.Contains(t, got.ExcludeText, "must be based in germany")
	require.Contains(t, got.Penalties, "work permit required")
	require.Contains(t, got.Penalties, "right to work in germany")
	for _, phrase := range []string{
		"must reside in the us",
		"must be based in the us",
		"authorized to work in the us",
		"authorization to work in the us",
		"us citizenship",
		"u.s. citizenship",
	} {
		require.Contains(t, got.Penalties, phrase)
		require.NotContains(t, got.ExcludeTitle, phrase)
		require.NotContains(t, got.ExcludeText, phrase)
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
	_, err := NewFileStore(dir, contractSourceIDs).Get(context.Background(), "sample")
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
	_, err := NewFileStore(dir, contractSourceIDs).Get(context.Background(), "sample")
	require.ErrorIs(t, err, profiles.ErrProfileInvalid)
}

func TestFileStoreGetReadsFileAgain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := NewFileStore(dir, contractSourceIDs)
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
	_, err := NewFileStore(t.TempDir(), contractSourceIDs).Get(context.Background(), "missing")
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
