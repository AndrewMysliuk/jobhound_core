package djinni

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
	category        string
	jobLocationType string
	baseSalaryRaw   json.RawMessage
	applicantLoc    json.RawMessage
	jobLocation     json.RawMessage
	ldTags          []string
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

func jsonLDIsJobPosting(raw json.RawMessage) bool {
	var probe struct {
		AtType json.RawMessage `json:"@type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return rawTypeContains(probe.AtType, "JobPosting")
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

func decodeJobPosting(raw json.RawMessage) (*jobPostingParsed, error) {
	var w struct {
		Title                string          `json:"title"`
		URL                  string          `json:"url"`
		Description          string          `json:"description"`
		DatePosted           string          `json:"datePosted"`
		Category             string          `json:"category"`
		JobLocationType      string          `json:"jobLocationType"`
		HiringOrganization   json.RawMessage `json:"hiringOrganization"`
		BaseSalary           json.RawMessage `json:"baseSalary"`
		ApplicantLocationReq json.RawMessage `json:"applicantLocationRequirements"`
		JobLocation          json.RawMessage `json:"jobLocation"`
		Skills               json.RawMessage `json:"skills"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, fmt.Errorf("JobPosting: %w", err)
	}
	company := hiringOrgName(w.HiringOrganization)
	var ldTags []string
	for _, t := range skillsTokens(w.Skills) {
		ldTags = append(ldTags, t)
	}
	return &jobPostingParsed{
		title:           strings.TrimSpace(w.Title),
		company:         company,
		rawURL:          strings.TrimSpace(w.URL),
		description:     w.Description,
		datePosted:      strings.TrimSpace(w.DatePosted),
		category:        strings.TrimSpace(w.Category),
		jobLocationType: strings.TrimSpace(w.JobLocationType),
		baseSalaryRaw:   append(json.RawMessage(nil), w.BaseSalary...),
		applicantLoc:    append(json.RawMessage(nil), w.ApplicantLocationReq...),
		jobLocation:     append(json.RawMessage(nil), w.JobLocation...),
		ldTags:          ldTags,
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

func skillsTokens(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return []string{strings.TrimSpace(s)}
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

func countryFromJobPosting(p *jobPostingParsed, cr *utils.CountryResolver) string {
	if p == nil {
		return ""
	}
	if c := countryFromApplicantOrLocation(p.applicantLoc, cr); c != "" {
		return c
	}
	return countryFromApplicantOrLocation(p.jobLocation, cr)
}

func countryFromApplicantOrLocation(raw json.RawMessage, cr *utils.CountryResolver) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	if raw[0] == '[' {
		var blocks []json.RawMessage
		if json.Unmarshal(raw, &blocks) != nil {
			return ""
		}
		for _, b := range blocks {
			if c := countryFromPlaceLike(b, cr); c != "" {
				return c
			}
		}
		return ""
	}
	return countryFromPlaceLike(raw, cr)
}

func countryFromPlaceLike(raw json.RawMessage, cr *utils.CountryResolver) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if addr, ok := m["address"]; ok {
		if c := alpha2FromAddress(addr, cr); c != "" {
			return c
		}
	}
	if c, ok := m["addressCountry"]; ok {
		if code := countryCodeFromRaw(c, cr); code != "" {
			return code
		}
	}
	return ""
}

func alpha2FromAddress(raw json.RawMessage, cr *utils.CountryResolver) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if c, ok := m["addressCountry"]; ok {
		return countryCodeFromRaw(c, cr)
	}
	return ""
}

func countryCodeFromRaw(raw json.RawMessage, cr *utils.CountryResolver) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		s = strings.TrimSpace(s)
		if len(s) == 2 {
			return strings.ToUpper(s)
		}
		if cr != nil {
			return cr.Alpha2ForName(s)
		}
		return ""
	}
	return ""
}

func mergeTags(category string, ldTags []string, listingTags []string) []string {
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
	if strings.TrimSpace(category) != "" {
		add(category)
	}
	for _, t := range ldTags {
		add(t)
	}
	for _, t := range listingTags {
		add(t)
	}
	return out
}

func resolveRemote(jobLocType string, title, description string, tags []string, metaHints []string) *bool {
	if strings.EqualFold(strings.TrimSpace(jobLocType), "TELECOMMUTE") {
		t := true
		return &t
	}
	return utils.RemoteMVPRule(title, description, tags, metaHints...)
}
