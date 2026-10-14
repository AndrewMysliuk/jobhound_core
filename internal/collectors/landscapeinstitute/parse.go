package landscapeinstitute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

var jobPathRE = regexp.MustCompile(`(?i)^/job/([^/]+)/?$`)

type listRow struct {
	URL       string
	Workplace string
}

func parseListHTML(body []byte) ([]listRow, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("landscapeinstitute: list html: %w", err)
	}
	var rows []listRow
	seen := make(map[string]struct{})
	doc.Find("li.job_listing").Each(func(_ int, li *goquery.Selection) {
		href := jobHref(li)
		if href == "" {
			return
		}
		row, ok := listingFromHref(href)
		if !ok {
			return
		}
		if _, dup := seen[row.URL]; dup {
			return
		}
		seen[row.URL] = struct{}{}
		row.Workplace = workplaceFromClass(li.AttrOr("class", ""))
		rows = append(rows, row)
	})
	return rows, nil
}

func jobHref(li *goquery.Selection) string {
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
	return href
}

func listingFromHref(href string) (listRow, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return listRow{}, false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "jobs.landscapeinstitute.org"
	}
	if !strings.EqualFold(u.Host, "jobs.landscapeinstitute.org") {
		return listRow{}, false
	}
	if jobPathRE.FindStringSubmatch(u.Path) == nil {
		return listRow{}, false
	}
	u.Scheme = "https"
	u.Host = "jobs.landscapeinstitute.org"
	u.RawQuery = ""
	u.Fragment = ""
	listing, err := utils.CanonicalListingURL(u.String())
	if err != nil {
		return listRow{}, false
	}
	return listRow{URL: listing}, true
}

func workplaceFromClass(class string) string {
	var hybrid bool
	for _, token := range strings.Fields(class) {
		const prefix = "job_listing_region-"
		if !strings.HasPrefix(strings.ToLower(token), prefix) {
			continue
		}
		switch strings.ToLower(token[len(prefix):]) {
		case "remotely":
			return schema.LocationRemote
		case "hybrid", "homebased":
			hybrid = true
		}
	}
	if hybrid {
		return schema.LocationHybrid
	}
	return ""
}

func parseDetailHTML(body []byte, row listRow) (schema.Job, error) {
	jp, err := utils.FirstJobPostingFromHTML(string(body), "landscapeinstitute")
	if err != nil {
		return schema.Job{}, err
	}
	title := strings.TrimSpace(jp.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("landscapeinstitute: detail %s: empty title", row.URL)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return schema.Job{}, fmt.Errorf("landscapeinstitute: detail %s: %w", row.URL, err)
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
		ApplyURL:    firstMailto(doc),
		Description: utils.StripHTMLToPlainText(html.UnescapeString(jp.Description)),
		PostedAt:    parsePosted(jp.DatePosted),
		SalaryRaw:   salaryFromDoc(doc),
		Location: schema.Location{
			Type:      row.Workplace,
			Countries: countries,
			Raw:       city,
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func firstMailto(doc *goquery.Document) string {
	var out string
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		if out != "" {
			return
		}
		href, _ := a.Attr("href")
		href = strings.TrimSpace(html.UnescapeString(href))
		if strings.HasPrefix(strings.ToLower(href), "mailto:") {
			out = href
		}
	})
	return out
}

func salaryFromDoc(doc *goquery.Document) string {
	var salary string
	found := false
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		if found {
			return
		}
		got, ok := salaryFromJSON(s.Text())
		if !ok {
			return
		}
		found = true
		salary = got
	})
	return salary
}

func salaryFromJSON(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "", false
	}
	posting := findJobPosting(v)
	if posting == nil {
		return "", false
	}
	return nestedSalary(posting["baseSalary"]), true
}

func findJobPosting(v any) map[string]any {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			if m := findJobPosting(item); m != nil {
				return m
			}
		}
	case map[string]any:
		if isJobPosting(t) {
			return t
		}
		if g, ok := t["@graph"]; ok {
			return findJobPosting(g)
		}
	}
	return nil
}

func isJobPosting(m map[string]any) bool {
	switch t := m["@type"].(type) {
	case string:
		return strings.EqualFold(t, "JobPosting")
	case []any:
		for _, item := range t {
			s, ok := item.(string)
			if ok && strings.EqualFold(s, "JobPosting") {
				return true
			}
		}
	}
	return false
}

func nestedSalary(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	inner, ok := m["value"].(map[string]any)
	if !ok {
		return ""
	}
	text, ok := inner["value"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func placeFromJobLocation(raw json.RawMessage) (string, string) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
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
		Address json.RawMessage `json:"address"`
	}
	if err := json.Unmarshal(raw, &place); err != nil {
		return "", ""
	}
	addr := bytes.TrimSpace(place.Address)
	if len(addr) == 0 || bytes.Equal(addr, []byte("null")) {
		return "", ""
	}
	if addr[0] == '"' {
		var text string
		if err := json.Unmarshal(addr, &text); err != nil {
			return "", ""
		}
		return strings.TrimSpace(text), ""
	}
	var obj struct {
		AddressLocality string          `json:"addressLocality"`
		AddressCountry  json.RawMessage `json:"addressCountry"`
	}
	if err := json.Unmarshal(addr, &obj); err != nil {
		return "", ""
	}
	return strings.TrimSpace(obj.AddressLocality), countryCode(obj.AddressCountry)
}

func countryCode(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
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
