package golangcafe

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

func testCountriesResolver(t *testing.T) *utils.CountryResolver {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	p := filepath.Join(repoRoot, "data", "countries.json")
	f, err := os.Open(p)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	r, err := utils.LoadCountryResolver(f)
	require.NoError(t, err)
	return r
}

func TestJobsFromPosts_locationAndRemoteKeep(t *testing.T) {
	posts := []jobPost{
		{ID: "1", Title: "Europe role", Company: "Acme", Remote: "remote", Country: "EU", Location: "Remote - Europe"},
		{ID: "2", Title: "On site Berlin", Company: "Acme", Remote: "on_site", Country: "DE", Location: "Berlin, Germany"},
		{ID: "3", Title: "Partial", Company: "Acme", Remote: "partially_remote", Country: "DE", Location: "Germany"},
		{ID: "4", Title: "On site remote word", Company: "Acme", Remote: "on_site", Country: "EU", Location: "Remote"},
	}
	jobs, err := jobsFromPosts(posts, 0, testCountriesResolver(t))
	require.NoError(t, err)
	require.Len(t, jobs, 2)

	require.Equal(t, "Europe role", jobs[0].Title)
	require.Empty(t, jobs[0].CountryCode)
	require.Empty(t, jobs[0].HiringCountries)
	require.Equal(t, []string{schema.RegionCodeEurope.String()}, jobs[0].HiringRegions)
	require.Equal(t, "Remote - Europe", jobs[0].HiringRaw)

	require.Equal(t, "Partial", jobs[1].Title)
	require.Equal(t, "DE", jobs[1].CountryCode)
	require.Equal(t, []string{"DE"}, jobs[1].HiringCountries)
}
