package baunetz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

var (
	jobPathRE = regexp.MustCompile(`(?i)^/stellenmarkt/job/[^/]+/[0-9]+$`)
	ldJSONRE  = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	stateMark = []byte("window.__INITIAL_STATE__=")
)

type jobDetail struct {
	Title               string          `json:"title"`
	Location            string          `json:"location"`
	Company             jobCompany      `json:"company"`
	PublicationDatetime string          `json:"publicationDatetime"`
	TextList            []jobText       `json:"textList"`
	ContactAddress      contactAddress  `json:"contactAddress"`
	BaseSalary          json.RawMessage `json:"baseSalary"`
}

type jobCompany struct {
	Title string `json:"title"`
}

type jobText struct {
	Text string `json:"text"`
}

type contactAddress struct {
	ApplicantPortal string `json:"applicantPortal"`
}

func parseListHTML(body []byte) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("baunetz: list html: %w", err)
	}
	var links []string
	seen := make(map[string]struct{})
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		link, ok := listingFromHref(href)
		if !ok {
			return
		}
		if _, dup := seen[link]; dup {
			return
		}
		seen[link] = struct{}{}
		links = append(links, link)
	})
	return links, nil
}

func listingFromHref(href string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil || u == nil {
		return "", false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "www.baunetz.de"
	}
	host := strings.ToLower(u.Host)
	if host != "www.baunetz.de" && host != "baunetz.de" {
		return "", false
	}
	path := u.Path
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	if !jobPathRE.MatchString(path) {
		return "", false
	}
	u.Scheme = "https"
	u.Host = "www.baunetz.de"
	u.Path = path
	u.RawQuery = ""
	u.Fragment = ""
	listing, err := utils.CanonicalListingURL(u.String())
	if err != nil {
		return "", false
	}
	return listing, true
}

func parseDetail(body []byte, listing string) (schema.Job, error) {
	detail, ok, err := detailFromState(body)
	if err != nil {
		return schema.Job{}, err
	}
	if ok {
		return jobFromDetail(detail, listing)
	}
	return jobFromJSONLD(body, listing)
}

func detailFromState(body []byte) (jobDetail, bool, error) {
	raw, ok, err := initialState(body)
	if err != nil || !ok {
		return jobDetail{}, false, err
	}
	var page struct {
		NonPersistentStore struct {
			JobDetailsPageContent *jobDetail `json:"jobDetailsPageContent"`
		} `json:"nonPersistentStore"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return jobDetail{}, false, fmt.Errorf("baunetz: detail state: %w", err)
	}
	if page.NonPersistentStore.JobDetailsPageContent == nil {
		return jobDetail{}, false, nil
	}
	return *page.NonPersistentStore.JobDetailsPageContent, true, nil
}

func initialState(body []byte) (json.RawMessage, bool, error) {
	i := bytes.Index(body, stateMark)
	if i < 0 {
		return nil, false, nil
	}
	dec := json.NewDecoder(bytes.NewReader(body[i+len(stateMark):]))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, false, fmt.Errorf("baunetz: detail state: %w", err)
	}
	return raw, true, nil
}

func jobFromDetail(detail jobDetail, listing string) (schema.Job, error) {
	title := strings.TrimSpace(detail.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("baunetz: detail %s: empty title", listing)
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     strings.TrimSpace(detail.Company.Title),
		URL:         listing,
		ApplyURL:    httpApplyURL(detail.ContactAddress.ApplicantPortal),
		Description: descriptionFromTexts(detail.TextList),
		PostedAt:    parsePosted(detail.PublicationDatetime),
		SalaryRaw:   salaryRaw(detail.BaseSalary),
		Location: schema.Location{
			Raw: strings.TrimSpace(detail.Location),
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func descriptionFromTexts(list []jobText) string {
	var b strings.Builder
	for _, item := range list {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(text)
	}
	return utils.StripHTMLToPlainText(b.String())
}

func jobFromJSONLD(body []byte, listing string) (schema.Job, error) {
	jp, err := utils.FirstJobPostingFromHTML(string(body), "baunetz")
	if err != nil {
		return schema.Job{}, err
	}
	title := strings.TrimSpace(jp.Title)
	if title == "" {
		return schema.Job{}, fmt.Errorf("baunetz: detail %s: empty title", listing)
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     strings.TrimSpace(jp.Company),
		URL:         listing,
		Description: utils.StripHTMLToPlainText(jp.Description),
		PostedAt:    parsePosted(jp.DatePosted),
		SalaryRaw:   salaryFromLD(body),
		Location: schema.Location{
			Raw: cityFromHiringOrg(jp.HiringOrg),
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func cityFromHiringOrg(raw json.RawMessage) string {
	var org struct {
		Location struct {
			AddressLocality string `json:"addressLocality"`
		} `json:"location"`
	}
	if err := json.Unmarshal(raw, &org); err != nil {
		return ""
	}
	return strings.TrimSpace(org.Location.AddressLocality)
}

func salaryRaw(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err != nil || n == 0 {
		return ""
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func salaryFromLD(body []byte) string {
	for _, m := range ldJSONRE.FindAllSubmatch(body, -1) {
		if len(m) < 2 {
			continue
		}
		var v any
		if err := json.Unmarshal(m[1], &v); err != nil {
			continue
		}
		if s, ok := salaryFromValue(v); ok {
			return s
		}
	}
	return ""
}

func salaryFromValue(v any) (string, bool) {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			if s, ok := salaryFromValue(item); ok {
				return s, true
			}
		}
	case map[string]any:
		if isJobPostingType(t["@type"]) {
			return salaryNumber(t["baseSalary"]), true
		}
		if g, ok := t["@graph"]; ok {
			return salaryFromValue(g)
		}
	}
	return "", false
}

func isJobPostingType(v any) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "JobPosting")
	case []any:
		for _, item := range t {
			s, ok := item.(string)
			if ok && strings.EqualFold(strings.TrimSpace(s), "JobPosting") {
				return true
			}
		}
	}
	return false
}

func salaryNumber(v any) string {
	n, ok := v.(float64)
	if !ok || n == 0 {
		return ""
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func httpApplyURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil || u == nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return s
}

func parsePosted(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
