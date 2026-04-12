package builtin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

type jobPostingParsed struct {
	title           string
	company         string
	rawURL          string
	description     string
	datePosted      string
	jobLocationType string
	baseSalaryRaw   json.RawMessage
	hiringOrg       json.RawMessage
	skills          json.RawMessage
	qualifications  json.RawMessage
	occupationalCat json.RawMessage
	employmentType  json.RawMessage
}

func parseListingJobURLs(html string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("listing document: %w", err)
	}
	var out []string
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		txt := strings.TrimSpace(s.Text())
		if txt == "" {
			return
		}
		u, _ := extractItemListURLsFromJSONLD([]byte(txt))
		out = append(out, u...)
	})
	return out, nil
}

func extractItemListURLsFromJSONLD(raw []byte) ([]string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	switch raw[0] {
	case '[':
		var blocks []json.RawMessage
		if err := json.Unmarshal(raw, &blocks); err != nil {
			return nil, nil
		}
		var out []string
		for _, b := range blocks {
			out = append(out, urlsFromItemListBlock(b)...)
		}
		return out, nil
	case '{':
		var wrap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrap); err != nil {
			return nil, nil
		}
		if g, ok := wrap["@graph"]; ok {
			var blocks []json.RawMessage
			if json.Unmarshal(g, &blocks) == nil {
				var out []string
				for _, b := range blocks {
					out = append(out, urlsFromItemListBlock(b)...)
				}
				if len(out) > 0 {
					return out, nil
				}
			}
		}
		return urlsFromItemListBlock(raw), nil
	default:
		return nil, nil
	}
}

func jsonLDIsItemList(raw json.RawMessage) bool {
	var probe struct {
		AtType json.RawMessage `json:"@type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return rawTypeContains(probe.AtType, "ItemList")
}

func urlsFromItemListBlock(raw json.RawMessage) []string {
	if !jsonLDIsItemList(raw) {
		return nil
	}
	var list struct {
		ItemListElement []json.RawMessage `json:"itemListElement"`
	}
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var urls []string
	for _, el := range list.ItemListElement {
		if u := listItemURL(el); u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

func listItemURL(raw json.RawMessage) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if u, ok := m["url"]; ok {
		var s string
		if json.Unmarshal(u, &s) == nil {
			return strings.TrimSpace(s)
		}
	}
	if it, ok := m["item"]; ok {
		var s string
		if json.Unmarshal(it, &s) == nil {
			return strings.TrimSpace(s)
		}
		var im struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(it, &im) == nil {
			return strings.TrimSpace(im.URL)
		}
	}
	return ""
}

func rawTypeContains(atType json.RawMessage, want string) bool {
	if len(atType) == 0 {
		return false
	}
	var s string
	if json.Unmarshal(atType, &s) == nil {
		return strings.EqualFold(strings.TrimSpace(s), want)
	}
	var ss []string
	if json.Unmarshal(atType, &ss) == nil {
		for _, x := range ss {
			if strings.EqualFold(strings.TrimSpace(x), want) {
				return true
			}
		}
	}
	return false
}

func jsonLDIsJobPosting(raw json.RawMessage) bool {
	var probe struct {
		AtType json.RawMessage `json:"@type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return rawTypeContains(probe.AtType, "JobPosting")
}

func parseJobPostingFromDetailHTML(html string) (*jobPostingParsed, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("detail document: %w", err)
	}
	var parseErr error
	var found *jobPostingParsed
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		if parseErr != nil || found != nil {
			return
		}
		txt := strings.TrimSpace(s.Text())
		if txt == "" {
			return
		}
		jp, err := extractFirstJobPosting([]byte(txt))
		if err != nil {
			parseErr = err
			return
		}
		if jp != nil {
			found = jp
		}
	})
	if parseErr != nil {
		return nil, parseErr
	}
	if found == nil {
		return nil, fmt.Errorf("detail: no JobPosting in application/ld+json")
	}
	return found, nil
}

func extractFirstJobPosting(raw []byte) (*jobPostingParsed, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	switch raw[0] {
	case '[':
		var blocks []json.RawMessage
		if err := json.Unmarshal(raw, &blocks); err != nil {
			return nil, fmt.Errorf("ld+json array: %w", err)
		}
		for _, b := range blocks {
			if jsonLDIsJobPosting(b) {
				return decodeJobPosting(b)
			}
		}
		return nil, nil
	case '{':
		var wrap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrap); err != nil {
			return nil, fmt.Errorf("ld+json object: %w", err)
		}
		if g, ok := wrap["@graph"]; ok {
			var blocks []json.RawMessage
			if err := json.Unmarshal(g, &blocks); err == nil {
				for _, b := range blocks {
					if jsonLDIsJobPosting(b) {
						return decodeJobPosting(b)
					}
				}
			}
		}
		if jsonLDIsJobPosting(raw) {
			return decodeJobPosting(raw)
		}
		return nil, nil
	default:
		return nil, nil
	}
}

func decodeJobPosting(raw json.RawMessage) (*jobPostingParsed, error) {
	var w struct {
		Title                string          `json:"title"`
		URL                  string          `json:"url"`
		Description          string          `json:"description"`
		DatePosted           string          `json:"datePosted"`
		JobLocationType      string          `json:"jobLocationType"`
		HiringOrganization   json.RawMessage `json:"hiringOrganization"`
		BaseSalary           json.RawMessage `json:"baseSalary"`
		Skills               json.RawMessage `json:"skills"`
		Qualifications       json.RawMessage `json:"qualifications"`
		OccupationalCategory json.RawMessage `json:"occupationalCategory"`
		EmploymentType       json.RawMessage `json:"employmentType"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, fmt.Errorf("JobPosting: %w", err)
	}
	return &jobPostingParsed{
		title:           strings.TrimSpace(w.Title),
		company:         hiringOrgName(w.HiringOrganization),
		rawURL:          strings.TrimSpace(w.URL),
		description:     w.Description,
		datePosted:      strings.TrimSpace(w.DatePosted),
		jobLocationType: strings.TrimSpace(w.JobLocationType),
		baseSalaryRaw:   append(json.RawMessage(nil), w.BaseSalary...),
		hiringOrg:       append(json.RawMessage(nil), w.HiringOrganization...),
		skills:          append(json.RawMessage(nil), w.Skills...),
		qualifications:  append(json.RawMessage(nil), w.Qualifications...),
		occupationalCat: append(json.RawMessage(nil), w.OccupationalCategory...),
		employmentType:  append(json.RawMessage(nil), w.EmploymentType...),
	}, nil
}

func hiringOrgName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var direct struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &direct) == nil && strings.TrimSpace(direct.Name) != "" {
		return strings.TrimSpace(direct.Name)
	}
	var nested struct {
		Name json.RawMessage `json:"name"`
	}
	if json.Unmarshal(raw, &nested) == nil && len(nested.Name) > 0 {
		var s string
		if json.Unmarshal(nested.Name, &s) == nil {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func applyURLFromHiringOrg(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var m struct {
		SameAs json.RawMessage `json:"sameAs"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	return firstHTTPURLFromRaw(m.SameAs)
}

func firstHTTPURLFromRaw(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(strings.ToLower(s), "http://") || strings.HasPrefix(strings.ToLower(s), "https://") {
			return s
		}
		return ""
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		for _, x := range ss {
			x = strings.TrimSpace(x)
			if strings.HasPrefix(strings.ToLower(x), "http://") || strings.HasPrefix(strings.ToLower(x), "https://") {
				return x
			}
		}
	}
	return ""
}

func builtinTags(p *jobPostingParsed) []string {
	if p == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" {
			return
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	for _, t := range jsonLDStringSlice(p.skills) {
		add(t)
	}
	for _, t := range jsonLDStringSlice(p.qualifications) {
		add(t)
	}
	for _, t := range jsonLDStringSlice(p.occupationalCat) {
		add(t)
	}
	return out
}

func jsonLDStringSlice(raw json.RawMessage) []string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		return []string{s}
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		var out []string
		for _, x := range ss {
			x = strings.TrimSpace(x)
			if x != "" {
				out = append(out, x)
			}
		}
		return out
	}
	return nil
}

func employmentTypeHint(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		return strings.Join(ss, ", ")
	}
	return ""
}

func plainFromJSONLDDescription(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "<") {
		return utils.NormalizePlainText(raw)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<body>" + raw + "</body>"))
	if err != nil {
		return utils.NormalizePlainText(raw)
	}
	return utils.NormalizePlainText(doc.Find("body").Text())
}

func formatSalaryRaw(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var w struct {
		Currency string          `json:"currency"`
		Value    json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return ""
	}
	cur := strings.TrimSpace(w.Currency)
	var val struct {
		MinValue json.RawMessage `json:"minValue"`
		MaxValue json.RawMessage `json:"maxValue"`
		UnitText string          `json:"unitText"`
	}
	if err := json.Unmarshal(w.Value, &val); err != nil {
		return ""
	}
	minN, minOK := jsonNumberString(val.MinValue)
	maxN, maxOK := jsonNumberString(val.MaxValue)
	if !minOK && !maxOK {
		return ""
	}
	unit := unitTextHuman(strings.TrimSpace(val.UnitText))
	var amount string
	switch {
	case minOK && maxOK && minN == maxN:
		amount = minN
	case minOK && maxOK:
		amount = minN + "–" + maxN
	case minOK:
		amount = minN
	default:
		amount = maxN
	}
	if cur != "" && unit != "" {
		return amount + " " + cur + " / " + unit
	}
	if cur != "" {
		return amount + " " + cur
	}
	if unit != "" {
		return amount + " / " + unit
	}
	return amount
}

func jsonNumberString(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return "", false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if math.Abs(f-math.Round(f)) < 1e-9 {
			return strconv.FormatInt(int64(math.Round(f)), 10), true
		}
		s := strconv.FormatFloat(f, 'f', -1, 64)
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		return s, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s), true
	}
	return "", false
}

func unitTextHuman(u string) string {
	switch strings.ToUpper(u) {
	case "MONTH", "MONTHLY":
		return "month"
	case "YEAR", "YEARLY":
		return "year"
	case "HOUR", "HOURLY":
		return "hour"
	case "WEEK", "WEEKLY":
		return "week"
	default:
		if u == "" {
			return ""
		}
		return strings.ToLower(u)
	}
}

func resolveRemote(p *jobPostingParsed, tags []string) *bool {
	if p == nil {
		f := false
		return &f
	}
	if strings.EqualFold(strings.TrimSpace(p.jobLocationType), "TELECOMMUTE") {
		t := true
		return &t
	}
	desc := plainFromJSONLDDescription(p.description)
	hint := employmentTypeHint(p.employmentType)
	var hints []string
	if hint != "" {
		hints = append(hints, hint)
	}
	return utils.RemoteMVPRule(p.title, desc, tags, hints...)
}
