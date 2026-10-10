package vuejobs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

func TestHiringScope_remoteCountries(t *testing.T) {
	t.Run("copies alpha-2 and ignores work place and locations", func(t *testing.T) {
		row, ok := listingJobFromRow(nil, map[string]any{
			"id":               "1",
			"slug":             "go-dev",
			"title":            "Go Dev",
			"organization":     map[string]any{"name": "Acme", "domain": "acme.example"},
			"apply_url":        "https://ats.example/apply/1",
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
		require.Equal(t, []string{"DE", "NL"}, jobs[0].Location.Countries)
		require.Empty(t, jobs[0].Location.Regions)
		require.Empty(t, jobs[0].Location.Raw)
		require.Equal(t, schema.LocationRemote, jobs[0].Location.Type)
		require.Equal(t, "https://ats.example/apply/1", jobs[0].ApplyURL)
		require.Equal(t, "https://acme.example", jobs[0].CompanyWebsite)
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
		require.Empty(t, jobs[0].Location.Countries)
		require.Empty(t, jobs[0].Location.Regions)
		require.Empty(t, jobs[0].Location.Raw)
		require.Equal(t, schema.LocationRemote, jobs[0].Location.Type)
		require.Empty(t, jobs[0].ApplyURL)
		require.Empty(t, jobs[0].CompanyWebsite)
	})
}
