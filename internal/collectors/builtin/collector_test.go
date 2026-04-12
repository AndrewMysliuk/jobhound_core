package builtin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type errRoundTrip struct{}

func (errRoundTrip) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected outbound HTTP")
}

func TestBuiltIn_Fetch_noHTTP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := &BuiltIn{
		HTTPClient: &http.Client{Transport: errRoundTrip{}},
	}
	jobs, err := c.Fetch(ctx)
	require.NoError(t, err)
	require.Empty(t, jobs)

	jobs, err = c.FetchWithSlotSearch(ctx, "  \t  ")
	require.NoError(t, err)
	require.Empty(t, jobs)
}

type seqHTMLFetcher struct {
	bodies [][]byte
	i      int
}

func (s *seqHTMLFetcher) FetchHTMLDocument(ctx context.Context, rawURL string) ([]byte, error) {
	if s.i >= len(s.bodies) {
		return nil, fmt.Errorf("unexpected fetch %d for %q", s.i, rawURL)
	}
	b := s.bodies[s.i]
	s.i++
	return b, nil
}

func TestBuiltIn_UseBrowser_withoutFetcher_errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := &BuiltIn{
		HTTPClient: &http.Client{Transport: errRoundTrip{}},
		UseBrowser: true,
	}
	_, err := c.FetchWithSlotSearch(ctx, "go")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTMLDocumentFetcher")
}

func TestBuiltIn_FetchWithSlotSearch_browserFetcher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	listingHTML := `<html><head><title>Remote</title></head><body>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "ItemList",
      "name": "Top Remote Tech Jobs",
      "numberOfItems": 1,
      "itemListElement": [
        {
          "@type": "ListItem",
          "position": 1,
          "name": "Senior Magento Frontend Developer",
          "url": "https://example.test/job/senior-magento-frontend-developer/8989543",
          "description": "Short snippet only."
        }
      ]
    }
  ]
}
</script>
</body></html>`

	detailHTML := `<html><body>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "JobPosting",
      "title": "Senior Magento Frontend Developer",
      "url": "https://example.test/job/senior-magento-frontend-developer/8989543",
      "description": "<p>Responsible for developing frontend components.</p>",
      "datePosted": "2026-04-08",
      "hiringOrganization": { "@type": "Organization", "name": "Xebia" },
      "jobLocationType": "TELECOMMUTE",
      "skills": ["Magento 2", "React", "GraphQL"]
    }
  ]
}
</script>
<a id="applyButton" href="https://example.test/apply/8989543">Apply</a>
</body></html>`

	listBase, err := url.Parse("https://example.test/jobs/remote")
	require.NoError(t, err)

	c := &BuiltIn{
		HTTPClient:                &http.Client{Transport: errRoundTrip{}},
		ListingBase:               listBase,
		InterRequestDelay:         0,
		TestAlpha3:                []string{"ROU"},
		MaxListingPagesPerCountry: 1,
		UseBrowser:                true,
		HTMLDocumentFetcher:       &seqHTMLFetcher{bodies: [][]byte{[]byte(listingHTML), []byte(detailHTML)}},
	}

	jobs, err := c.FetchWithSlotSearch(ctx, "go")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "Senior Magento Frontend Developer", jobs[0].Title)
	require.Equal(t, "https://example.test/apply/8989543", jobs[0].ApplyURL)
}

func TestBuiltIn_FetchWithSlotSearch_httptest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	listingHTML := `<html><head><title>Remote</title></head><body>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "ItemList",
      "name": "Top Remote Tech Jobs",
      "numberOfItems": 1,
      "itemListElement": [
        {
          "@type": "ListItem",
          "position": 1,
          "name": "Senior Magento Frontend Developer",
          "url": "%s/job/senior-magento-frontend-developer/8989543",
          "description": "Short snippet only."
        }
      ]
    }
  ]
}
</script>
</body></html>`

	detailHTML := `<html><body>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "JobPosting",
      "title": "Senior Magento Frontend Developer",
      "url": "%s/job/senior-magento-frontend-developer/8989543",
      "description": "<p>Responsible for developing frontend components.</p>",
      "datePosted": "2026-04-08",
      "hiringOrganization": { "@type": "Organization", "name": "Xebia" },
      "jobLocationType": "TELECOMMUTE",
      "skills": ["Magento 2", "React", "GraphQL"]
    }
  ]
}
</script>
<a id="applyButton" href="%s/apply/8989543">Apply</a>
</body></html>`

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := strings.TrimSuffix(ts.URL, "/")
		switch {
		case r.URL.Path == "/jobs/remote":
			require.Equal(t, "ROU", r.URL.Query().Get("country"))
			require.Equal(t, "true", r.URL.Query().Get("allLocations"))
			require.Equal(t, "go", r.URL.Query().Get("search"))
			require.Equal(t, "1", r.URL.Query().Get("page"))
			_, _ = w.Write([]byte(fmt.Sprintf(listingHTML, base)))
		case strings.HasPrefix(r.URL.Path, "/job/"):
			_, _ = w.Write([]byte(fmt.Sprintf(detailHTML, base, base)))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	root, err := url.Parse(ts.URL)
	require.NoError(t, err)
	listBase := *root
	listBase.Path = "/jobs/remote"
	listBase.RawQuery = ""

	c := &BuiltIn{
		HTTPClient:                ts.Client(),
		ListingBase:               &listBase,
		InterRequestDelay:         0,
		TestAlpha3:                []string{"ROU"},
		MaxListingPagesPerCountry: 2,
	}

	jobs, err := c.FetchWithSlotSearch(ctx, "go")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	j := jobs[0]
	require.Equal(t, SourceName, j.Source)
	require.Equal(t, "Senior Magento Frontend Developer", j.Title)
	require.Equal(t, "Xebia", j.Company)
	require.Equal(t, "RO", j.CountryCode)
	require.Contains(t, j.URL, "/job/senior-magento-frontend-developer/8989543")
	require.Contains(t, j.URL, "8989543")
	require.NotNil(t, j.Remote)
	require.True(t, *j.Remote)
	require.Equal(t, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), j.PostedAt.UTC())
	require.Contains(t, j.Description, "Responsible for developing frontend components")
	require.Equal(t, []string{"Magento 2", "React", "GraphQL"}, j.Tags)
	require.NotEmpty(t, j.ID)
	require.Equal(t, ts.URL+"/apply/8989543", j.ApplyURL)
}
