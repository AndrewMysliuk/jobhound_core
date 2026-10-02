package vuejobs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHiringScope_remoteCountries(t *testing.T) {
	t.Run("copies alpha-2 and ignores work place and locations", func(t *testing.T) {
		row, ok := listingJobFromRow(nil, map[string]any{
			"id":               "1",
			"slug":             "go-dev",
			"title":            "Go Dev",
			"organization":     map[string]any{"name": "Acme"},
			"work_place":       []any{"remote"},
			"remote_countries": []any{"de", "NL", "Germany", "de"},
			"locations":        []any{"Berlin", "United States"},
		})
		require.True(t, ok)
		require.Equal(t, []string{"remote"}, row.WorkPlace)
		require.Equal(t, []string{"de", "NL", "Germany", "de"}, row.RemoteCountries)

		jobs, err := jobsFromListing([]ListingJob{row})
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		require.Equal(t, []string{"DE", "NL"}, jobs[0].HiringCountries)
		require.Empty(t, jobs[0].HiringRegions)
		require.Empty(t, jobs[0].HiringRaw)
		require.Equal(t, "DE", jobs[0].CountryCode)
		require.NotNil(t, jobs[0].Remote)
		require.True(t, *jobs[0].Remote)
	})

	t.Run("empty remote countries stay not stated", func(t *testing.T) {
		row, ok := listingJobFromRow(nil, map[string]any{
			"id":               "2",
			"slug":             "empty",
			"title":            "Empty",
			"organization":     map[string]any{"name": "Acme"},
			"work_place":       []any{"remote"},
			"remote_countries": []any{},
			"locations":        []any{"Germany", "Berlin"},
		})
		require.True(t, ok)
		require.Empty(t, row.RemoteCountries)

		jobs, err := jobsFromListing([]ListingJob{row})
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		require.Empty(t, jobs[0].HiringCountries)
		require.Empty(t, jobs[0].HiringRegions)
		require.Empty(t, jobs[0].HiringRaw)
		require.NotNil(t, jobs[0].Remote)
		require.True(t, *jobs[0].Remote)
	})
}
