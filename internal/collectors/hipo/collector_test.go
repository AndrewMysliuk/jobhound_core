package hipo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func TestFetchWithQuery_categoryPagesAndDetail(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case categoryPath:
			writePage(t, w, listPage([]card{
				{id: "200001", company: "Firm", slug: "Senior---Architect", title: "Senior Architect"},
				{id: promoListingID, company: "Top-Talents-Romania", slug: "Inscrie-te", title: "Promo"},
				{id: "200002", company: "PORR", slug: "Inginer-proiectare", title: "Inginer proiectare"},
			}, 3))
		case categoryPath + "/2":
			writePage(t, w, listPage([]card{
				{id: "200003", company: "Studio", slug: "Peisagist", title: "Peisagist"},
			}, 3))
		case categoryPath + "/3":
			writePage(t, w, listPage([]card{
				{id: "200004", company: "Confidential", slug: "Arhitect", title: "Arhitect"},
			}, 3))
		case "/locuri-de-munca/locuri_de_munca/200001/Firm/Senior-Architect":
			writePage(t, w, detailPage(detailFixture{
				title: "Senior Architect", companyLabel: "Employer:", company: "Firm",
				placeLabel: "Location:", place: "BUCURESTI",
				work: "Remote", updated: "Actualizat la:",
				bodyClass: "alertbox2", body: "<p>Visible <b>description</b></p>",
				other: `<div class="content"><p>Should not win</p></div>`,
			}))
		case "/locuri-de-munca/locuri_de_munca/200002/PORR/Inginer-proiectare":
			writePage(t, w, detailPage(detailFixture{
				title: "Inginer proiectare", companyLabel: "Angajator:", company: "PORR",
				placeLabel: "Orase:", place: "Iasi",
				work: "Hybrid", updated: "Actualizat la:",
				bodyClass: "content", body: "<p>Proiectare <b>cladiri</b></p>",
			}))
		case "/locuri-de-munca/locuri_de_munca/200003/Studio/Peisagist":
			writePage(t, w, detailPage(detailFixture{
				title: "Peisagist", companyLabel: "Employer:", company: "Studio",
				placeLabel: "Location:", place: "Cluj",
				work: "On-site", updated: "Updated at:",
				bodyClass: "content", body: "<p>Office</p>",
				salaryLD: "9000 EUR",
			}))
		case "/locuri-de-munca/locuri_de_munca/200004/Confidential/Arhitect":
			writePage(t, w, detailPage(detailFixture{
				title: "Arhitect", companyLabel: "Employer:", company: "Confidențial",
				work: "Remote", bodyClass: "alertbox2", body: "<p>Remote studio</p>",
			}))
		default:
			t.Errorf("unexpected path %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Hipo{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, &agents)}}
	jobs, err := c.FetchWithQuery(context.Background(), "cuvintecheie arhitect")
	require.NoError(t, err)
	require.Equal(t, SourceName, c.Name())

	var lists []string
	for _, raw := range urls {
		require.NotContains(t, raw, "cuvintecheie")
		require.NotContains(t, raw, "---")
		require.NotContains(t, raw, promoListingID)
		if strings.Contains(raw, categoryPath) {
			lists = append(lists, raw)
		}
	}
	require.Equal(t, []string{
		categoryURL,
		categoryURL + "/2",
		categoryURL + "/3",
	}, lists)
	for _, agent := range agents {
		require.Equal(t, utils.BrowserUserAgent, agent)
	}

	require.Len(t, jobs, 4)

	remote := jobs[0]
	require.Equal(t, "Senior Architect", remote.Title)
	require.Equal(t, "Firm", remote.Company)
	require.Equal(t, "https://www.hipo.ro/locuri-de-munca/locuri_de_munca/200001/Firm/Senior-Architect", remote.URL)
	require.Equal(t, "Visible description", remote.Description)
	require.NotContains(t, remote.Description, "JSON-LD")
	require.NotContains(t, remote.Description, "Probleme")
	require.NotContains(t, remote.Description, "Should not win")
	require.Empty(t, remote.ApplyURL)
	require.Empty(t, remote.SalaryRaw)
	require.True(t, remote.PostedAt.IsZero())
	require.Equal(t, schema.LocationRemote, remote.Location.Type)
	require.Equal(t, []string{"RO"}, remote.Location.Countries)
	require.Equal(t, "BUCURESTI", remote.Location.Raw)
	require.Equal(t, domainutils.StableJobID(remote.CompanyKey, remote.Title), remote.ID)
	require.NotContains(t, remote.ID, remote.URL)

	hybrid := jobs[1]
	require.Equal(t, "Proiectare cladiri", hybrid.Description)
	require.NotContains(t, hybrid.Description, "Probleme")
	require.Equal(t, schema.LocationHybrid, hybrid.Location.Type)
	require.Equal(t, "Iasi", hybrid.Location.Raw)
	require.Equal(t, "PORR", hybrid.Company)
	require.True(t, hybrid.PostedAt.IsZero())
	require.Empty(t, hybrid.SalaryRaw)
	require.Empty(t, hybrid.ApplyURL)

	office := jobs[2]
	require.Equal(t, schema.LocationOffice, office.Location.Type)
	require.Empty(t, office.SalaryRaw)
	require.NotContains(t, office.Description, "9000")
	require.True(t, office.PostedAt.IsZero())

	placeholder := jobs[3]
	require.Equal(t, "Arhitect", placeholder.Title)
	require.Contains(t, placeholder.ID, placeholder.URL)
}

func TestFetch_noPagerStopsOnEmptyPage(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case categoryPath:
			writePage(t, w, listPage([]card{
				{id: "200010", company: "Firm", slug: "Arhitect", title: "Arhitect"},
			}, 0))
		case categoryPath + "/2":
			writePage(t, w, `<html><body><div class="job-item"><h1>0 Locuri de munca</h1></div></body></html>`)
		case "/locuri-de-munca/locuri_de_munca/200010/Firm/Arhitect":
			writePage(t, w, detailPage(detailFixture{
				title: "Arhitect", companyLabel: "Employer:", company: "Firm",
				work: "On-site", bodyClass: "content", body: "<p>One page</p>",
			}))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := &Hipo{HTTPClient: &http.Client{Transport: recordHost(srv.URL, &mu, &urls, nil)}}
	jobs, err := c.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "One page", jobs[0].Description)

	var lists []string
	for _, raw := range urls {
		if strings.Contains(raw, categoryPath) {
			lists = append(lists, raw)
		}
	}
	require.Equal(t, []string{categoryURL, categoryURL + "/2"}, lists)
}

type card struct {
	id      string
	company string
	slug    string
	title   string
}

func listPage(cards []card, last int) string {
	var b strings.Builder
	b.WriteString(`<html><body>`)
	b.WriteString(`<input name="cuvintecheie" value="ignored">`)
	b.WriteString(`<a href="https://www.hipo.ro/locuri-de-munca/cautajob/Arhitectura---Design---Proiectare">triple</a>`)
	b.WriteString(`<div class="job-item"><h1>31 Locuri de munca</h1></div>`)
	for _, card := range cards {
		b.WriteString(`<div class="job-item"><a class="job-title" href="/locuri-de-munca/locuri_de_munca/`)
		b.WriteString(card.id)
		b.WriteString(`/`)
		b.WriteString(card.company)
		b.WriteString(`/`)
		b.WriteString(card.slug)
		b.WriteString(`">`)
		b.WriteString(card.title)
		b.WriteString(`</a></div>`)
	}
	if last > 1 {
		b.WriteString(`<a class="page page-last" href="`)
		b.WriteString(categoryURL)
		b.WriteString(`/`)
		b.WriteString(strconv.Itoa(last))
		b.WriteString(`"></a>`)
	}
	b.WriteString(`</body></html>`)
	return b.String()
}

type detailFixture struct {
	title        string
	companyLabel string
	company      string
	placeLabel   string
	place        string
	work         string
	updated      string
	bodyClass    string
	body         string
	other        string
	salaryLD     string
}

func detailPage(f detailFixture) string {
	var b strings.Builder
	b.WriteString(`<html><body>`)
	b.WriteString(`<div id="ment-floater"><div class="site__text alertbox2">Probleme tehnice</div></div>`)
	b.WriteString(`<div class="main-content"><h1 class="titlu-articol">`)
	b.WriteString(f.title)
	b.WriteString(`</h1><table>`)
	if f.companyLabel != "" {
		b.WriteString(`<tr><td>`)
		b.WriteString(f.companyLabel)
		b.WriteString(`</td><td><a>`)
		b.WriteString(f.company)
		b.WriteString(`</a></td></tr>`)
	}
	if f.placeLabel != "" {
		b.WriteString(`<tr><td>`)
		b.WriteString(f.placeLabel)
		b.WriteString(`</td><td><a>`)
		b.WriteString(f.place)
		b.WriteString(`</a></td></tr>`)
	}
	if f.updated != "" {
		b.WriteString(`<tr><td>`)
		b.WriteString(f.updated)
		b.WriteString(`</td><td>14-10-2026</td></tr>`)
	}
	if f.work != "" {
		b.WriteString(`<tr><td>Remote work:</td><td><b>`)
		b.WriteString(f.work)
		b.WriteString(`</b></td></tr>`)
	}
	b.WriteString(`</table>`)
	b.WriteString(`<a href="https://www.hipo.ro/locuri-de-munca/login?redirect=/apply">Apply to this job</a>`)
	b.WriteString(`<div class="`)
	b.WriteString(f.bodyClass)
	b.WriteString(`">`)
	b.WriteString(f.body)
	b.WriteString(`</div>`)
	b.WriteString(f.other)
	b.WriteString(`</div>`)
	b.WriteString(`<script type="application/ld+json">{"description":"JSON-LD text","baseSalary":"`)
	b.WriteString(f.salaryLD)
	b.WriteString(`","datePosted":"2026-10-14","jobLocationType":"TELECOMMUTE"}</script>`)
	b.WriteString(`</body></html>`)
	return b.String()
}

func writePage(t *testing.T, w http.ResponseWriter, html string) {
	t.Helper()
	_, err := w.Write([]byte(html))
	require.NoError(t, err)
}

type recordTransport struct {
	baseURL string
	next    http.RoundTripper
	mu      *sync.Mutex
	urls    *[]string
	agents  *[]string
}

func recordHost(base string, mu *sync.Mutex, urls *[]string, agents *[]string) recordTransport {
	return recordTransport{baseURL: base, next: http.DefaultTransport, mu: mu, urls: urls, agents: agents}
}

func (r recordTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	*r.urls = append(*r.urls, req.URL.String())
	if r.agents != nil {
		*r.agents = append(*r.agents, req.Header.Get("User-Agent"))
	}
	r.mu.Unlock()
	target, err := url.Parse(r.baseURL)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.URL.Scheme = target.Scheme
	clone.URL.Host = target.Host
	clone.Host = target.Host
	return r.next.RoundTrip(clone)
}
