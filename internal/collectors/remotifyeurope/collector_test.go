package remotifyeurope

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

//go:embed testdata/listing_header_pin.html
var listingHeaderPinHTML string

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

func TestHiringScope_headerPinWhenJSONLDEmpty(t *testing.T) {
	detail, err := ParseListingDetailHTML(listingHeaderPinHTML)
	require.NoError(t, err)
	require.Empty(t, detail.LocationTexts)
	require.Equal(t, "United Kingdom", detail.HeaderPin)
	require.Contains(t, detail.Description, "France")

	code, countries, regions, raw := hiringScopeFromDetail(testCountriesResolver(t), detail)
	require.Equal(t, "GB", code)
	require.Equal(t, []string{"GB"}, countries)
	require.Empty(t, regions)
	require.Equal(t, "United Kingdom", raw)
	require.NotContains(t, countries, "FR")
}
