package dezeen

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

var jobPathRE = regexp.MustCompile(`(?i)^/job/(.+)-(\d+)/?$`)

type listRow struct {
	ID     string
	URL    string
	Remote bool
}

func parseListHTML(body []byte) ([]listRow, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("dezeen: list html: %w", err)
	}
	var rows []listRow
	seen := make(map[string]struct{})
	doc.Find(".main-jobs-list li.job_listing").Each(func(_ int, li *goquery.Selection) {
		var row listRow
		var ok bool
		li.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
			if ok {
				return
			}
			href, _ := a.Attr("href")
			row, ok = listingFromHref(href)
		})
		if !ok {
			return
		}
		if _, dup := seen[row.ID]; dup {
			return
		}
		seen[row.ID] = struct{}{}
		row.Remote = locationTermRemote(li)
		rows = append(rows, row)
	})
	return rows, nil
}

func listingFromHref(href string) (listRow, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return listRow{}, false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "www.dezeenjobs.com"
	}
	host := strings.ToLower(u.Host)
	if host != "www.dezeenjobs.com" && host != "dezeenjobs.com" {
		return listRow{}, false
	}
	match := jobPathRE.FindStringSubmatch(u.Path)
	if match == nil {
		return listRow{}, false
	}
	u.Scheme = "https"
	u.Host = "www.dezeenjobs.com"
	u.RawQuery = ""
	u.Fragment = ""
	listing, err := utils.CanonicalListingURL(u.String())
	if err != nil {
		return listRow{}, false
	}
	return listRow{ID: match[2], URL: listing}, true
}

func parseDetailHTML(body []byte, row listRow) (schema.Job, error) {
	jp, err := utils.FirstJobPostingFromHTML(string(body), "dezeen")
	if err != nil {
		return schema.Job{}, err
	}
	title := strings.TrimSpace(jp.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("dezeen: detail %s: empty title", row.URL)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return schema.Job{}, fmt.Errorf("dezeen: detail %s: %w", row.URL, err)
	}
	city, country := placeFromJobLocation(jp.JobLocation)
	var countries []string
	if country != "" {
		countries = []string{country}
	}
	locType := ""
	if row.Remote || detailRemote(doc) {
		locType = schema.LocationRemote
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     strings.TrimSpace(jp.Company),
		URL:         row.URL,
		ApplyURL:    applyHref(doc, row.URL),
		Description: utils.StripHTMLToPlainText(jp.Description),
		PostedAt:    parsePosted(jp.DatePosted),
		SalaryRaw:   utils.NormalizePlainText(doc.Find("div.salary-range").First().Text()),
		Location: schema.Location{
			Type:      locType,
			Countries: countries,
			Raw:       city,
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func detailRemote(doc *goquery.Document) bool {
	if doc.Find(".single_job_listing").Length() > 0 {
		return locationTermRemote(doc.Find(".single_job_listing"))
	}
	return locationTermRemote(doc.Find(".location"))
}

func locationTermRemote(root *goquery.Selection) bool {
	if root.Length() == 0 {
		return false
	}
	found := false
	root.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		if locationTerm(href) == "remote" {
			found = true
		}
	})
	if found {
		return true
	}
	root.Find(".location").Each(func(_ int, s *goquery.Selection) {
		if strings.EqualFold(utils.NormalizePlainText(s.Text()), "remote") {
			found = true
		}
	})
	return found
}

func locationTerm(href string) string {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], "location") {
			return strings.ToLower(parts[i+1])
		}
	}
	return ""
}

func applyHref(doc *goquery.Document, listing string) string {
	block := doc.Find(".job_application").First()
	if block.Length() == 0 {
		return ""
	}
	if applyBlocked(block.Text()) {
		return ""
	}
	var href string
	block.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		if href != "" {
			return
		}
		raw, _ := a.Attr("href")
		got := httpApply(raw)
		if got == "" || got == listing {
			return
		}
		href = got
	})
	return href
}

func applyBlocked(text string) bool {
	folded := strings.ToLower(text)
	return strings.Contains(folded, "sign-in") ||
		strings.Contains(folded, "sign in") ||
		strings.Contains(folded, "geo gate") ||
		strings.Contains(folded, "geogate")
}

func httpApply(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	if u.Host == "" {
		return ""
	}
	return u.String()
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
