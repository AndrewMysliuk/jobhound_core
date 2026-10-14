package riba

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

var jobPathRE = regexp.MustCompile(`(?i)^/job/(\d+)/([^/]+)/?$`)

type listRow struct {
	ID  string
	URL string
}

func parseListHTML(body []byte) ([]listRow, bool, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("riba: list html: %w", err)
	}
	var rows []listRow
	seen := make(map[string]struct{})
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		row, ok := listingFromHref(href)
		if !ok {
			return
		}
		if _, dup := seen[row.ID]; dup {
			return
		}
		seen[row.ID] = struct{}{}
		rows = append(rows, row)
	})
	return rows, hasRelNext(doc), nil
}

func hasRelNext(doc *goquery.Document) bool {
	found := false
	doc.Find("link").Each(func(_ int, s *goquery.Selection) {
		rel, ok := s.Attr("rel")
		if !ok {
			return
		}
		for _, part := range strings.Fields(rel) {
			if strings.EqualFold(part, "next") {
				found = true
			}
		}
	})
	return found
}

func listingFromHref(href string) (listRow, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return listRow{}, false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "jobs.architecture.com"
	}
	if !strings.EqualFold(u.Host, "jobs.architecture.com") {
		return listRow{}, false
	}
	match := jobPathRE.FindStringSubmatch(u.Path)
	if match == nil {
		return listRow{}, false
	}
	u.Scheme = "https"
	u.Host = "jobs.architecture.com"
	u.RawQuery = ""
	u.Fragment = ""
	listing, err := utils.CanonicalListingURL(u.String())
	if err != nil {
		return listRow{}, false
	}
	return listRow{ID: match[1], URL: listing}, true
}

func parseDetailHTML(body []byte, row listRow) (schema.Job, error) {
	jp, err := utils.FirstJobPostingFromHTML(string(body), "riba")
	if err != nil {
		return schema.Job{}, err
	}
	title := strings.TrimSpace(jp.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("riba: detail %s: empty title", row.URL)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return schema.Job{}, fmt.Errorf("riba: detail %s: %w", row.URL, err)
	}
	city, country := placeFromJobLocation(jp.JobLocation)
	var countries []string
	if country != "" {
		countries = []string{country}
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     strings.TrimSpace(jp.Company),
		URL:         row.URL,
		ApplyURL:    row.URL,
		Description: utils.StripHTMLToPlainText(jp.Description),
		PostedAt:    parsePosted(jp.DatePosted),
		SalaryRaw:   salaryRow(doc),
		Location: schema.Location{
			Countries: countries,
			Raw:       city,
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func salaryRow(doc *goquery.Document) string {
	var salary string
	doc.Find("dt").Each(func(_ int, dt *goquery.Selection) {
		if salary != "" {
			return
		}
		label := strings.Trim(utils.NormalizePlainText(dt.Text()), ":")
		if !strings.EqualFold(label, "salary") {
			return
		}
		dd := dt.NextFiltered("dd")
		if dd.Length() == 0 {
			dd = dt.Parent().Find("dd").First()
		}
		salary = utils.NormalizePlainText(dd.Text())
	})
	return salary
}

func placeFromJobLocation(raw json.RawMessage) (string, string) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return "", ""
	}
	if raw[0] == '[' {
		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil {
			return "", ""
		}
		for _, part := range parts {
			city, country := placeFromJobLocation(part)
			if city != "" || country != "" {
				return city, country
			}
		}
		return "", ""
	}
	var place struct {
		Address struct {
			AddressLocality string          `json:"addressLocality"`
			AddressCountry  json.RawMessage `json:"addressCountry"`
		} `json:"address"`
	}
	if err := json.Unmarshal(raw, &place); err != nil {
		return "", ""
	}
	return strings.TrimSpace(place.Address.AddressLocality), countryCode(place.Address.AddressCountry)
}

func countryCode(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return normalizeCountry(text)
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return normalizeCountry(obj.Name)
	}
	return ""
}

func normalizeCountry(raw string) string {
	s := strings.TrimSpace(raw)
	if len(s) == 2 {
		return strings.ToUpper(s)
	}
	return s
}

func parsePosted(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
