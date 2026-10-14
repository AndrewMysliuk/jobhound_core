package hipo

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

var jobIDRE = regexp.MustCompile(`/locuri-de-munca/locuri_de_munca/(\d+)(?:/|$)`)

type listRow struct {
	ID  string
	URL string
}

func parseListHTML(body []byte) ([]listRow, int, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("hipo: list html: %w", err)
	}
	var rows []listRow
	seen := make(map[string]struct{})
	doc.Find("a.job-title").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		row, ok := listingFromHref(href)
		if !ok {
			return
		}
		if row.ID == promoListingID {
			return
		}
		if _, dup := seen[row.URL]; dup {
			return
		}
		seen[row.URL] = struct{}{}
		rows = append(rows, row)
	})
	return rows, lastPage(doc), nil
}

func lastPage(doc *goquery.Document) int {
	href, ok := doc.Find("a.page.page-last").Attr("href")
	if !ok {
		return 0
	}
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return 0
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 {
		return 0
	}
	n, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || n < 1 {
		return 0
	}
	return n
}

func listingFromHref(href string) (listRow, bool) {
	raw := strings.TrimSpace(href)
	if raw == "" || strings.Contains(raw, "cuvintecheie") {
		return listRow{}, false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return listRow{}, false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "www.hipo.ro"
	}
	host := strings.ToLower(u.Host)
	if host != "www.hipo.ro" && host != "hipo.ro" {
		return listRow{}, false
	}
	match := jobIDRE.FindStringSubmatch(u.Path)
	if match == nil {
		return listRow{}, false
	}
	u.Scheme = "https"
	u.Host = "www.hipo.ro"
	u.Path = strings.ReplaceAll(u.Path, "---", "-")
	u.RawQuery = ""
	u.Fragment = ""
	listing, err := utils.CanonicalListingURL(u.String())
	if err != nil {
		return listRow{}, false
	}
	return listRow{ID: match[1], URL: listing}, true
}

func parseDetailHTML(body []byte, listing string) (schema.Job, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return schema.Job{}, fmt.Errorf("hipo: detail %s: %w", listing, err)
	}
	main := doc.Find("div.main-content")
	title := strings.TrimSpace(main.Find("h1.titlu-articol").First().Text())
	if title == "" {
		return schema.Job{}, fmt.Errorf("hipo: detail %s: empty title", listing)
	}
	company := rowText(main, "employer", "angajator")
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         listing,
		Description: descriptionOf(main),
		Location: schema.Location{
			Type:      workplace(rowText(main, "remote work")),
			Countries: []string{"RO"},
			Raw:       rowText(main, "location", "orase", "orașe", "oras", "oraș"),
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

// descriptionOf reads the posting body. div.alertbox2 inside the posting wins.
// The same class on the site footer is a support note, and current pages put
// the posting in div.content. JSON-LD is not read.
func descriptionOf(main *goquery.Selection) string {
	if text := selectionText(main.Find("div.alertbox2").First()); text != "" {
		return text
	}
	return selectionText(main.Find("div.content").First())
}

func selectionText(s *goquery.Selection) string {
	if s.Length() == 0 {
		return ""
	}
	html, err := s.Html()
	if err != nil {
		return ""
	}
	return utils.StripHTMLToPlainText(html)
}

func rowText(main *goquery.Selection, labels ...string) string {
	want := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		want[label] = struct{}{}
	}
	var found string
	main.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		cells := tr.Find("td")
		if cells.Length() < 2 || found != "" {
			return
		}
		label := strings.Trim(strings.ToLower(strings.TrimSpace(cells.Eq(0).Text())), ":")
		if _, ok := want[label]; !ok {
			return
		}
		found = utils.NormalizePlainText(cells.Eq(1).Text())
	})
	return found
}

func workplace(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "remote":
		return schema.LocationRemote
	case "hybrid":
		return schema.LocationHybrid
	case "on-site", "on site", "onsite":
		return schema.LocationOffice
	default:
		return ""
	}
}
