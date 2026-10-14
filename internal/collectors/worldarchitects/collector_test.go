package worldarchitects

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/browserfetch"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetch_catalogStopsOnRepeatedSlugs(t *testing.T) {
	var urls []string
	c := &WorldArchitects{Fetcher: &fakeFetcher{
		urls: &urls,
		pages: map[string]string{
			siteOrigin + listPath:             listPage1,
			siteOrigin + listPath + "?page=2": listPage2,
			siteOrigin + listPath + "?page=3": listPage3,
			siteOrigin + listPath + "/alpha":  detailNamed,
			siteOrigin + listPath + "/beta":   detailConfidential,
			siteOrigin + listPath + "/gamma":  detailGermany,
		},
	}}
	jobs, err := c.FetchWithQuery(context.Background(), "catalog")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())
	require.Len(t, jobs, 3)

	require.Equal(t, []string{
		siteOrigin + listPath,
		siteOrigin + listPath + "/alpha",
		siteOrigin + listPath + "/beta",
		siteOrigin + listPath + "?page=2",
		siteOrigin + listPath + "/gamma",
		siteOrigin + listPath + "?page=3",
	}, urls)
	for _, raw := range urls {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.False(t, queryHasFold(u, "term"))
		require.NotContains(t, raw, "arhitect")
		require.NotContains(t, raw, "page=4")
		require.NotContains(t, raw, "page=9")
	}
	named := jobs[0]
	require.Equal(t, "Projektleiter Architektur", named.Title)
	require.Equal(t, "ffbk Architekten AG", named.Company)
	require.Equal(t, siteOrigin+listPath+"/alpha", named.URL)
	require.Empty(t, named.ApplyURL)
	require.Equal(t, "Planen und bauen im Büro.", named.Description)
	require.Empty(t, named.SalaryRaw)
	require.Empty(t, named.Location.Type)
	require.Equal(t, []string{"CH"}, named.Location.Countries)
	require.Equal(t, "Zürich", named.Location.Raw)
	require.Equal(t, time.Date(2026, 10, 14, 9, 41, 0, 0, time.UTC), named.PostedAt)
	require.Equal(t, domainutils.StableJobID(named.CompanyKey, named.Title), named.ID)
	require.NotContains(t, named.ID, named.URL)
	require.NotContains(t, named.Description, "Building of the Week")
	require.NotContains(t, named.Description, "90000")

	placeholder := jobs[1]
	require.Equal(t, "Architekt:in", placeholder.Title)
	require.Equal(t, "Confidential", placeholder.Company)
	require.Empty(t, placeholder.ApplyURL)
	require.Empty(t, placeholder.SalaryRaw)
	require.Empty(t, placeholder.Location.Type)
	require.Equal(t, []string{"CH"}, placeholder.Location.Countries)
	require.Equal(t, "Aarau", placeholder.Location.Raw)
	require.Equal(t, "Studio", placeholder.Description)
	require.Empty(t, placeholder.PostedAt)
	norm, err := domainutils.NormalizeListingURL(placeholder.URL)
	require.NoError(t, err)
	require.Contains(t, placeholder.ID, norm)
	require.NotEqual(t, domainutils.StableJobID(placeholder.CompanyKey, placeholder.Title), placeholder.ID)

	german := jobs[2]
	require.Equal(t, "Bauleiter", german.Title)
	require.Equal(t, "Schmelzle Partner", german.Company)
	require.Equal(t, "Freiraum in Hallwangen.", german.Description)
	require.Equal(t, []string{"DE"}, german.Location.Countries)
	require.Equal(t, "Hallwangen", german.Location.Raw)
	require.Empty(t, german.SalaryRaw)
	require.Empty(t, german.ApplyURL)
	require.Empty(t, german.Location.Type)
	require.Equal(t, time.Date(2026, 10, 12, 13, 28, 0, 0, time.UTC), german.PostedAt)
	require.NotContains(t, german.ID, german.URL)
	require.NotContains(t, german.ID, "IGNORE")
}

func TestFetchWithQuery_ignoresQueryAndSkipsTerm(t *testing.T) {
	var urls []string
	empty := `<form action="/en/jobs"><input name="term" value="arhitect"></form><a href="/en/jobs?term=arhitect">search</a><a href="/en/jobs?page=9">9</a>`
	c := &WorldArchitects{Fetcher: &fakeFetcher{
		urls: &urls,
		pages: map[string]string{
			siteOrigin + listPath:             empty,
			siteOrigin + listPath + "?page=2": empty,
		},
	}}
	for _, query := range []string{"", "catalog", "term=foo", "Architekt"} {
		jobs, err := c.FetchWithQuery(context.Background(), query)
		require.NoError(t, err)
		require.Empty(t, jobs)
	}
	fetched, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Empty(t, fetched)
	require.Len(t, urls, 10)
	for i, raw := range urls {
		if i%2 == 0 {
			require.Equal(t, siteOrigin+listPath, raw)
		} else {
			require.Equal(t, siteOrigin+listPath+"?page=2", raw)
		}
		require.NotContains(t, raw, "term=")
		require.NotContains(t, raw, "Architekt")
		require.NotContains(t, raw, "page=9")
	}
}

func TestFetch_emptyTitle(t *testing.T) {
	c := &WorldArchitects{Fetcher: &fakeFetcher{pages: map[string]string{
		siteOrigin + listPath:            `<li class="row"><a href="/en/jobs/empty">x</a></li>`,
		siteOrigin + listPath + "/empty": `<h1>  </h1><script type="application/ld+json">{"@type":"JobPosting","title":"Real"}</script>`,
	}}}
	_, err := c.Fetch(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "empty title")
}

func TestFetch_nilFetcher(t *testing.T) {
	_, err := (&WorldArchitects{}).Fetch(context.Background())
	require.ErrorContains(t, err, "browser fetcher is nil")
}

func TestGet_refusesTerm(t *testing.T) {
	c := &WorldArchitects{Fetcher: &fakeFetcher{}}
	for _, raw := range []string{
		siteOrigin + listPath + "?term=arhitect",
		siteOrigin + listPath + "?page=2&term=Architekt",
		siteOrigin + listPath + "?TERM=foo",
	} {
		_, err := c.get(context.Background(), raw)
		require.ErrorContains(t, err, "refused")
	}
}

func queryHasFold(u *url.URL, key string) bool {
	for name := range u.Query() {
		if strings.EqualFold(name, key) {
			return true
		}
	}
	return false
}

const listPage1 = `
<form action="/en/jobs" method="post"><input name="term" value="arhitect"></form>
<a href="/en/jobs?term=arhitect">search</a>
<div class="well well-shadow row">
  <dl class="no-margin col-md-7">
    <dt><a href="http://world-architects.com/en/jobs/alpha?utm=1">Projektleiter</a></dt>
  </dl>
  <a class="btn" href="/en/jobs/alpha">View Top Job</a>
</div>
<ul>
<li class="row">
  <dl class="col-md-6">
    <dt><a href="/en/jobs/alpha">Projektleiter Architektur</a></dt>
    <dd><a href="/en/ffbk">ffbk Architekten AG</a></dd>
  </dl>
  <address>Zürich, Switzerland</address>
  <time datetime="10/14/26, 9:41 AM">14. October</time>
</li>
<li class="row">
  <dl>
    <dt><a href="https://www.world-architects.com/en/jobs/beta/">Architekt:in</a></dt>
    <dd>Confidential</dd>
  </dl>
  <address>Aarau, Switzerland</address>
</li>
</ul>
<div class="pagination">
  <a href="/en/jobs?page=2">2</a>
  <a href="/en/jobs?page=9">9</a>
</div>
`

const listPage2 = `
<li class="row">
  <a href="/en/jobs/gamma">Bauleiter</a>
  <time datetime="10/12/26, 1:28 PM">12. October</time>
</li>
<li class="row">
  <a href="/en/jobs/alpha">dup</a>
  <time datetime="10/14/26, 9:41 AM">14. October</time>
</li>
<a href="/en/jobs?page=4">4</a>
`

const listPage3 = `
<a href="/en/jobs/alpha">alpha</a>
<a href="/en/jobs/gamma">gamma</a>
<div class="pagination"><a href="/en/jobs?page=4">4</a></div>
`

const detailNamed = `
<script type="application/ld+json">{"@type":"JobPosting","title":"IGNORE","description":"nope","datePosted":"2020-01-01","baseSalary":90000}</script>
<div class="col-md-12">
  <h1>Projektleiter Architektur</h1>
  <h5><a href="/en/ffbk-architekten">ffbk Architekten AG <i class="wa-arrow"></i></a></h5>
</div>
<div class="col-md-7 m-b-3">
  <p>Planen und <b>bauen</b> im Büro.</p>
</div>
<dl>
  <dt>Provider</dt>
  <dd>ffbk Architekten AG<br>Basel<br>Switzerland<br>
    <a href="mailto:job@ffbk.example">mail</a>
    <a href="https://www.ffbk.example">www.ffbk.example</a>
  </dd>
  <dt>Job Location</dt>
  <dd>Zürich, Switzerland</dd>
  <dt>Date</dt>
  <dd><time datetime="1/1/20, 8:00 AM">14. October</time></dd>
</dl>
<div class="salary">90000 CHF</div>
<h2>Building of the Week</h2>
<p>Not the description</p>
`

const detailConfidential = `
<h1>Architekt:in</h1>
<h5>Confidential</h5>
<div class="col-md-7"><p>Studio</p></div>
<dl>
  <dt>Job Location</dt>
  <dd>Aarau, Switzerland</dd>
  <dt>Date</dt>
  <dd><time datetime="1/1/99, 8:00 AM">1. January</time></dd>
</dl>
<a href="https://apply.example/job">apply</a>
<a href="mailto:job@firm.example">mail</a>
`

const detailGermany = `
<script type="application/ld+json">{"title":"IGNORE","datePosted":"2020-01-02"}</script>
<h1>Bauleiter</h1>
<h5><a href="/en/schmelzle">Schmelzle Partner</a></h5>
<div class="col-md-7"><p>Freiraum in Hallwangen.</p></div>
<dl>
  <dt>Job Location</dt>
  <dd>Hallwangen, Germany</dd>
  <dt>Date</dt>
  <dd><time datetime="1/2/20, 3:04 AM">12. October</time></dd>
</dl>
<p><a href="https://jobs.example/apply">portal</a></p>
`

type fakeFetcher struct {
	pages map[string]string
	urls  *[]string
}

func (f *fakeFetcher) FetchNavigated(_ context.Context, rawURL string) (browserfetch.NavigatedDocument, error) {
	if f.urls != nil {
		*f.urls = append(*f.urls, rawURL)
	}
	html, ok := f.pages[rawURL]
	if !ok {
		return browserfetch.NavigatedDocument{}, fmt.Errorf("unexpected %s", rawURL)
	}
	return browserfetch.NavigatedDocument{FinalURL: rawURL, HTML: []byte(html)}, nil
}
