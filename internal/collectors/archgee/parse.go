package archgee

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

var jobPathRE = regexp.MustCompile(`(?i)^/jobs/([^/]+)/?$`)

type listRow struct {
	URL       string
	Workplace string
}

func parseListHTML(body []byte) ([]listRow, bool, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("archgee: list html: %w", err)
	}
	var rows []listRow
	seen := make(map[string]struct{})
	doc.Find("a.job-tile").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		row, ok := listingFromHref(href)
		if !ok {
			return
		}
		if _, dup := seen[row.URL]; dup {
			return
		}
		seen[row.URL] = struct{}{}
		row.Workplace = workplaceFromCard(a)
		rows = append(rows, row)
	})
	return rows, hasRelNext(doc), nil
}

func listingFromHref(href string) (listRow, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return listRow{}, false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "archgee.com"
	}
	host := strings.ToLower(u.Host)
	if host != "archgee.com" && host != "www.archgee.com" {
		return listRow{}, false
	}
	if strings.Contains(strings.ToLower(u.Path), "/api/") {
		return listRow{}, false
	}
	match := jobPathRE.FindStringSubmatch(u.Path)
	if match == nil || strings.EqualFold(match[1], "post") {
		return listRow{}, false
	}
	u.Scheme = "https"
	u.Host = "archgee.com"
	u.RawQuery = ""
	u.Fragment = ""
	listing, err := utils.CanonicalListingURL(u.String())
	if err != nil {
		return listRow{}, false
	}
	return listRow{URL: listing}, true
}

func workplaceFromCard(a *goquery.Selection) string {
	var out string
	a.Find("span").Each(func(_ int, s *goquery.Selection) {
		if out != "" {
			return
		}
		out = workplaceFromBadge(s.Text())
	})
	return out
}

func workplaceFromBadge(text string) string {
	switch strings.ToLower(utils.NormalizePlainText(text)) {
	case "remote":
		return schema.LocationRemote
	case "hybrid":
		return schema.LocationHybrid
	case "on-site", "onsite":
		return schema.LocationOffice
	default:
		return ""
	}
}

func hasRelNext(doc *goquery.Document) bool {
	found := false
	doc.Find("a[rel], link[rel]").Each(func(_ int, s *goquery.Selection) {
		rel, _ := s.Attr("rel")
		for _, token := range strings.Fields(rel) {
			if strings.EqualFold(token, "next") {
				found = true
			}
		}
	})
	return found
}

func parseDetailHTML(body []byte, row listRow) (schema.Job, error) {
	jp, err := utils.FirstJobPostingFromHTML(string(body), "archgee")
	if err != nil {
		return schema.Job{}, err
	}
	title := strings.TrimSpace(jp.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("archgee: detail %s: empty title", row.URL)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return schema.Job{}, fmt.Errorf("archgee: detail %s: %w", row.URL, err)
	}
	company := strings.TrimSpace(jp.Company)
	city, country := placeFromJobLocation(jp.JobLocation)
	var countries []string
	if country != "" {
		countries = []string{country}
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         row.URL,
		Description: utils.StripHTMLToPlainText(jp.Description),
		PostedAt:    parsePosted(jp.DatePosted),
		SalaryRaw:   salaryFromMeta(doc),
		Location: schema.Location{
			Type:      workplace(jp.JobLocationType, row.Workplace),
			Countries: countries,
			Raw:       city,
		},
	}
	// The identifier UUID is job.ID only before AssignStableID, and only for a placeholder company.
	if placeholderCompany(company) {
		out.ID = postingIdentifier(string(body))
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func placeholderCompany(name string) bool {
	switch domainutils.CompanyKey(name) {
	case "", "confidential", "confidențial", "nedivulgat":
		return true
	default:
		return false
	}
}

func workplace(jobLocationType, badge string) string {
	switch strings.ToUpper(strings.TrimSpace(jobLocationType)) {
	case "TELECOMMUTE":
		return schema.LocationRemote
	case "":
		return badge
	default:
		return ""
	}
}

func salaryFromMeta(doc *goquery.Document) string {
	content, ok := doc.Find(`meta[name="description"]`).First().Attr("content")
	if !ok {
		return ""
	}
	content = utils.NormalizePlainText(content)
	parts := strings.Split(content, " | ")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func postingIdentifier(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	var id string
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		if id != "" {
			return
		}
		id = identifierFromJSON(s.Text())
	})
	return id
}

func identifierFromJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return ""
	}
	posting := findJobPosting(v)
	if posting == nil {
		return ""
	}
	return identifierValue(posting["identifier"])
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

func identifierValue(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		s, _ := t["value"].(string)
		return strings.TrimSpace(s)
	default:
		return ""
	}
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
