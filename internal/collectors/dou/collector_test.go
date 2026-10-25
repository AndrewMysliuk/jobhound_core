package dou

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

func TestFeedURL(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", FeedURL(DefaultFeedBase, ""))
	require.Equal(t, "", FeedURL(DefaultFeedBase, "  "))
	require.Equal(t,
		"https://jobs.dou.ua/vacancies/feeds/?search=golang&remote",
		FeedURL(DefaultFeedBase, "golang"),
	)
	require.Equal(t,
		"https://jobs.dou.ua/vacancies/feeds/?search=vue.js&remote",
		FeedURL(DefaultFeedBase, "vue.js"),
	)
	require.Equal(t,
		"https://example.test/feeds/?search=node&remote",
		FeedURL("https://example.test/feeds/", "node"),
	)
}

func TestFeedItemsFromRSS(t *testing.T) {
	t.Parallel()
	body := []byte(`<?xml version="1.0"?>
<rss version="2.0"><channel>
<item>
  <title>Go Engineer at Acme — віддалено</title>
  <link>https://jobs.dou.ua/companies/acme/vacancies/123/?utm_source=jobsrss</link>
  <guid>https://jobs.dou.ua/companies/acme/vacancies/123/</guid>
  <pubDate>Sat, 25 Oct 2026 10:00:00 +0000</pubDate>
  <description><![CDATA[<p>Teaser about Go</p>]]></description>
</item>
<item>
  <title>Skipped empty</title>
  <link></link>
  <guid></guid>
</item>
</channel></rss>`)

	items, err := feedItemsFromRSS(body)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Go Engineer at Acme — віддалено", items[0].Title)
	require.Contains(t, items[0].Link, "/vacancies/123/")
	require.Equal(t, time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC), items[0].PubDate)
	require.Contains(t, items[0].Description, "Teaser about Go")
}

func TestParseDetailHTML_selectors(t *testing.T) {
	t.Parallel()
	html := []byte(`<!DOCTYPE html><html><body>
<h1 class="g-h2">Senior Golang Developer</h1>
<div class="b-compinfo"><div class="l-n"><a href="/companies/acme/">Acme Soft</a></div></div>
<div class="b-typo vacancy-section"><p>Build APIs in <b>Go</b>.</p></div>
<div class="place">віддалено</div>
<div class="date">21 жовтня 2026</div>
<a class="replied-external" href="https://dou.ua/goto/vacancy/?id=123">Відгукнутися</a>
</body></html>`)

	d, err := parseDetailHTML(html)
	require.NoError(t, err)
	require.Equal(t, "Senior Golang Developer", d.Title)
	require.Equal(t, "Acme Soft", d.Company)
	require.Contains(t, d.Description, "Build APIs in Go")
	require.Equal(t, "віддалено", d.PlaceRaw)
	require.Equal(t, time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), d.PostedAt)
	require.Equal(t, "https://dou.ua/goto/vacancy/?id=123", d.ApplyURL)
}

func TestApplyURLFromHref(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		href string
		want string
	}{
		{name: "external absolute", href: "https://dou.ua/goto/vacancy/?id=99", want: "https://dou.ua/goto/vacancy/?id=99"},
		{name: "external relative", href: "/goto/vacancy/?id=42", want: "https://dou.ua/goto/vacancy/?id=42"},
		{name: "on-dou hash", href: "#apply", want: ""},
		{name: "empty", href: "", want: ""},
		{name: "unrelated", href: "https://example.com/apply", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, applyURLFromHref(tc.href))
		})
	}
}

func TestJobFromFeedAndDetail_applyAndPostedAt(t *testing.T) {
	t.Parallel()
	rssPub := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	detailDate := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	baseItem := feedItem{
		Title:       "RSS Title at Co — meta",
		Link:        "https://jobs.dou.ua/companies/acme/vacancies/55/?utm_source=jobsrss",
		PubDate:     rssPub,
		Description: "<p>RSS teaser</p>",
	}
	baseDetail := detailFields{
		Title:       "Detail Title",
		Company:     "Acme",
		Description: "Full description remote",
		PlaceRaw:    "Львів, віддалено",
		PostedAt:    detailDate,
		ApplyURL:    "https://dou.ua/goto/vacancy/?id=55",
	}

	cases := []struct {
		name        string
		item        feedItem
		detail      *detailFields
		wantApply   string
		wantPosted  time.Time
		wantTitle   string
		wantCompany string
	}{
		{
			name:        "external apply and RSS pubDate preferred",
			item:        baseItem,
			detail:      &baseDetail,
			wantApply:   "https://dou.ua/goto/vacancy/?id=55",
			wantPosted:  rssPub,
			wantTitle:   "Detail Title",
			wantCompany: "Acme",
		},
		{
			name: "on-dou hash apply empty",
			item: baseItem,
			detail: func() *detailFields {
				d := baseDetail
				d.ApplyURL = applyURLFromHref("#apply")
				return &d
			}(),
			wantApply:   "",
			wantPosted:  rssPub,
			wantTitle:   "Detail Title",
			wantCompany: "Acme",
		},
		{
			name: "detail date when RSS pubDate zero",
			item: func() feedItem {
				it := baseItem
				it.PubDate = time.Time{}
				return it
			}(),
			detail:      &baseDetail,
			wantApply:   "https://dou.ua/goto/vacancy/?id=55",
			wantPosted:  detailDate,
			wantTitle:   "Detail Title",
			wantCompany: "Acme",
		},
		{
			name:        "nil detail RSS fallback",
			item:        baseItem,
			detail:      nil,
			wantApply:   "",
			wantPosted:  rssPub,
			wantTitle:   "RSS Title at Co — meta",
			wantCompany: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j, err := jobFromFeedAndDetail(tc.item, tc.detail)
			require.NoError(t, err)
			require.Equal(t, SourceName, j.Source)
			require.Equal(t, tc.wantTitle, j.Title)
			require.Equal(t, tc.wantCompany, j.Company)
			require.Equal(t, tc.wantApply, j.ApplyURL)
			require.Equal(t, tc.wantPosted, j.PostedAt)
			require.Equal(t, "https://jobs.dou.ua/companies/acme/vacancies/55", j.URL)
			require.NotEmpty(t, j.ID)
			if tc.detail != nil && tc.detail.PlaceRaw != "" {
				require.Equal(t, "Львів, віддалено", j.Location.Raw)
				require.Equal(t, schema.LocationRemote, j.Location.Type)
			}
			if tc.detail == nil {
				require.Contains(t, j.Description, "RSS teaser")
			}
		})
	}
}

func TestFetchWithQuery_emptyListFailAndSoftSkip(t *testing.T) {
	t.Parallel()

	t.Run("empty query", func(t *testing.T) {
		c := &Dou{}
		jobs, err := c.Fetch(context.Background())
		require.NoError(t, err)
		require.Empty(t, jobs)

		jobs, err = c.FetchWithQuery(context.Background(), "  ")
		require.NoError(t, err)
		require.Empty(t, jobs)
	})

	t.Run("list fail", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "nope", http.StatusBadGateway)
		}))
		t.Cleanup(srv.Close)

		c := &Dou{HTTPClient: srv.Client(), FeedBase: srv.URL + "/"}
		_, err := c.FetchWithQuery(context.Background(), "golang")
		require.Error(t, err)
		require.Contains(t, err.Error(), "FETCH")
	})

	softSkipCases := []struct {
		name       string
		detailBody string
		detailCode int
	}{
		{name: "detail HTTP error", detailCode: http.StatusInternalServerError, detailBody: "detail down"},
		{name: "bad detail HTML", detailCode: http.StatusOK, detailBody: "<html><body><p>not a vacancy</p></body></html>"},
	}
	for _, tc := range softSkipCases {
		t.Run(tc.name, func(t *testing.T) {
			var srv *httptest.Server
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.RawQuery, "search=") {
					w.Header().Set("Content-Type", "application/rss+xml")
					_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel>
<item>
  <title>Soft Job</title>
  <link>` + srv.URL + `/companies/x/vacancies/1/</link>
  <pubDate>Sat, 25 Oct 2026 10:00:00 +0000</pubDate>
  <description><![CDATA[Fallback body]]></description>
</item>
<item>
  <title>Bad URL</title>
  <link>not-a-url</link>
</item>
</channel></rss>`))
					return
				}
				w.WriteHeader(tc.detailCode)
				_, _ = w.Write([]byte(tc.detailBody))
			}))
			t.Cleanup(srv.Close)

			c := &Dou{HTTPClient: srv.Client(), FeedBase: srv.URL + "/"}
			jobs, err := c.FetchWithQuery(context.Background(), "golang")
			require.NoError(t, err)
			require.Len(t, jobs, 1)
			require.Equal(t, "Soft Job", jobs[0].Title)
			require.Contains(t, jobs[0].Description, "Fallback body")
			require.Empty(t, jobs[0].ApplyURL)
		})
	}

	t.Run("happy path detail map", func(t *testing.T) {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.RawQuery, "search=") {
				require.Equal(t, "application/rss+xml, application/xml, text/xml, */*", r.Header.Get("Accept"))
				require.NotEmpty(t, r.Header.Get("User-Agent"))
				require.Contains(t, r.URL.RawQuery, "remote")
				w.Header().Set("Content-Type", "application/rss+xml")
				_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel>
<item>
  <title>RSS Title</title>
  <link>` + srv.URL + `/companies/acme/vacancies/9/?utm_source=jobsrss</link>
  <pubDate>Sat, 25 Oct 2026 10:00:00 +0000</pubDate>
  <description>teaser</description>
</item>
</channel></rss>`))
				return
			}
			if strings.Contains(r.URL.Path, "/vacancies/9") {
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<h1>Vue Engineer</h1>
<div class="b-compinfo"><div class="l-n"><a href="/companies/acme/">Acme</a></div></div>
<div class="b-typo"><p>Work with Vue remotely</p></div>
<div class="place">віддалено</div>
<div class="date">20 жовтня 2026</div>
<a class="replied-external" href="/goto/vacancy/?id=9">go</a>
</body></html>`))
				return
			}
			http.NotFound(w, r)
		}))
		t.Cleanup(srv.Close)

		c := &Dou{HTTPClient: srv.Client(), FeedBase: srv.URL + "/"}
		jobs, err := c.FetchWithQuery(context.Background(), "vue")
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		require.Equal(t, "Vue Engineer", jobs[0].Title)
		require.Equal(t, "Acme", jobs[0].Company)
		require.Equal(t, srv.URL+"/companies/acme/vacancies/9", jobs[0].URL)
		require.Equal(t, "https://dou.ua/goto/vacancy/?id=9", jobs[0].ApplyURL)
		require.Equal(t, "віддалено", jobs[0].Location.Raw)
		require.Equal(t, time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC), jobs[0].PostedAt)
	})
}

func TestName(t *testing.T) {
	require.Equal(t, "dou_ua", (&Dou{}).Name())
}
