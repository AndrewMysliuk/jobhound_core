package archinect

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

var entryPathRE = regexp.MustCompile(`(?i)^/jobs/entry/(\d+)(?:/[^/]+)?/?$`)

type listRow struct {
	ID       string
	URL      string
	Title    string
	Company  string
	Location string
	Posted   string
	Salary   string
}

func parseListHTML(body []byte) ([]listRow, int, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("archinect: list html: %w", err)
	}
	var rows []listRow
	seen := make(map[string]struct{})
	doc.Find("li.Entry").Each(func(_ int, li *goquery.Selection) {
		row, ok := rowFromEntry(li)
		if !ok {
			return
		}
		if _, dup := seen[row.ID]; dup {
			return
		}
		seen[row.ID] = struct{}{}
		rows = append(rows, row)
	})
	return rows, totalCount(doc), nil
}

func rowFromEntry(li *goquery.Selection) (listRow, bool) {
	var href string
	li.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		if href != "" {
			return
		}
		raw, _ := a.Attr("href")
		if _, ok := listingFromHref(raw); ok {
			href = raw
		}
	})
	row, ok := listingFromHref(href)
	if !ok {
		return listRow{}, false
	}
	row.Title = utils.NormalizePlainText(li.Find("h1").First().Text())
	row.Company = utils.NormalizePlainText(li.Find("h2").First().Text())
	row.Location = utils.NormalizePlainText(li.Find("div.Col2 p").First().Text())
	row.Posted = utils.NormalizePlainText(li.Find("div.Col2 span").First().Text())
	row.Salary = payString(li.Find("h3").First().Text())
	return row, true
}

func listingFromHref(href string) (listRow, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return listRow{}, false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "archinect.com"
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host != "archinect.com" {
		return listRow{}, false
	}
	match := entryPathRE.FindStringSubmatch(u.Path)
	if match == nil {
		return listRow{}, false
	}
	u.Scheme = "https"
	u.Host = "archinect.com"
	u.RawQuery = ""
	u.Fragment = ""
	listing, err := utils.CanonicalListingURL(u.String())
	if err != nil {
		return listRow{}, false
	}
	return listRow{ID: match[1], URL: listing}, true
}

func totalCount(doc *goquery.Document) int {
	raw := strings.TrimSpace(doc.Find("#arc_tr").First().Text())
	if raw == "" {
		return -1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return -1
	}
	return n
}

func parseDetailHTML(body []byte, row listRow) (schema.Job, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return schema.Job{}, fmt.Errorf("archinect: detail %s: %w", row.URL, err)
	}
	box := doc.Find("div.JobDescription").First()
	info := box.Find("div.JobInfo").First()
	title := utils.NormalizePlainText(box.Find("h1").First().Text())
	if title == "" {
		title = row.Title
	}
	if title == "" {
		return schema.Job{}, fmt.Errorf("archinect: detail %s: empty title", row.URL)
	}
	company := infoValue(info, "Employer")
	if company == "" {
		company = row.Company
	}
	location := infoValue(info, "Location")
	if location == "" {
		location = row.Location
	}
	posted := infoValue(info, "Posted on")
	if posted == "" {
		posted = row.Posted
	}
	salary := payString(infoValue(info, "Pay"))
	if salary == "" {
		salary = row.Salary
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         row.URL,
		Description: descriptionText(box),
		PostedAt:    parsePosted(posted),
		SalaryRaw:   salary,
		Location: schema.Location{
			Type: schema.LocationRemote,
			Raw:  location,
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func infoValue(info *goquery.Selection, label string) string {
	if info == nil || info.Length() == 0 {
		return ""
	}
	want := strings.TrimSuffix(strings.ToLower(label), ":")
	var out string
	info.Find("span.Left").Each(func(_ int, s *goquery.Selection) {
		if out != "" {
			return
		}
		got := strings.TrimSuffix(strings.ToLower(utils.NormalizePlainText(s.Text())), ":")
		if got != want {
			return
		}
		prev := s.Prev()
		for prev.Length() > 0 && !prev.Is("span.Right") {
			prev = prev.Prev()
		}
		out = utils.NormalizePlainText(prev.Text())
	})
	return out
}

func descriptionText(box *goquery.Selection) string {
	if box == nil || box.Length() == 0 {
		return ""
	}
	clone := box.Clone()
	clone.Find(".Head, script, style").Remove()
	return utils.NormalizePlainText(clone.Text())
}

func payString(raw string) string {
	s := utils.NormalizePlainText(raw)
	for _, r := range s {
		if unicode.IsDigit(r) {
			return s
		}
	}
	return ""
}

func parsePosted(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{"Mon, Jan 2 '06", "Mon, Jan 2 2006", time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
