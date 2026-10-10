package weworkremotely

import (
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// WWR states the company site as <strong>URL:</strong> followed by one anchor. Other links in the description are not that field.
var companyURLRE = regexp.MustCompile(`(?i)<strong>\s*URL:\s*</strong>\s*<a\s[^>]*href="([^"]+)"`)

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Region      string `xml:"region"`
	Country     string `xml:"country"`
	State       string `xml:"state"`
	Skills      string `xml:"skills"`
	Category    string `xml:"category"`
	Type        string `xml:"type"`
}

func jobsFromRSS(body []byte, countries *utils.CountryResolver) ([]schema.Job, error) {
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decode RSS: %w", err)
	}
	var out []schema.Job
	for _, item := range feed.Channel.Items {
		j, ok, err := jobFromRSSItem(countries, item)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, j)
		}
	}
	return out, nil
}

func jobFromRSSItem(countries *utils.CountryResolver, item rssItem) (schema.Job, bool, error) {
	company, title := splitCompanyTitle(item.Title)
	if company == "" || title == "" {
		return schema.Job{}, false, nil
	}
	rawLink := strings.TrimSpace(item.Link)
	if rawLink == "" {
		return schema.Job{}, false, nil
	}
	listingURL, err := utils.CanonicalListingURL(rawLink)
	if err != nil {
		return schema.Job{}, false, fmt.Errorf("listing URL %q: %w", rawLink, err)
	}
	descHTML := strings.TrimSpace(item.Description)
	descPlain := utils.StripHTMLToPlainText(descHTML)
	companyWebsite := companyWebsiteFromDescription(descHTML)
	tags := skillsTags(item.Skills, item.Category)
	postedAt, _ := parseRSSPubDate(strings.TrimSpace(item.PubDate))
	// WWR states the hiring restriction in <region> ("Anywhere in the World", "USA Only", "Europe Only").
	// <country>/<state> is the company location, so they are only a fallback when <region> says nothing.
	hiringCountries, hiringRegions, hiringRaw := utils.ParseHiringScope(countries, item.Region)
	if len(hiringCountries) == 0 && len(hiringRegions) == 0 {
		hiringCountries, hiringRegions, hiringRaw = utils.ParseHiringScope(countries, item.Country, item.State)
	}
	j := schema.Job{
		Source:         SourceName,
		Title:          title,
		Company:        company,
		CompanyWebsite: companyWebsite,
		URL:            listingURL,
		Description:    descPlain,
		PostedAt:       postedAt,
		Location:       utils.LocationFromParsed(remoteFromWWR(item.Region, title, descPlain, tags), "", hiringCountries, hiringRegions, hiringRaw, nil),
		Tags:           tags,
		Position:       utils.InferPosition(title, descPlain, tags),
	}
	if err := domainutils.AssignStableID(&j); err != nil {
		return schema.Job{}, false, fmt.Errorf("stable id: %w", err)
	}
	return j, true, nil
}

func companyWebsiteFromDescription(descHTML string) string {
	m := companyURLRE.FindStringSubmatch(descHTML)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(m[1]))
}

func splitCompanyTitle(raw string) (company, title string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ""
	}
	if i := strings.Index(s, ": "); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+2:])
	}
	return "", s
}

func parseRSSPubDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty pubDate")
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse %q", s)
}

func skillsTags(skills, category string) []string {
	var out []string
	for _, part := range strings.Split(skills, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if cat := strings.TrimSpace(category); cat != "" {
		out = append(out, cat)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func remoteFromWWR(region, title, desc string, tags []string) *bool {
	region = strings.TrimSpace(region)
	if strings.Contains(strings.ToLower(region), "anywhere") {
		v := true
		return &v
	}
	return utils.RemoteMVPRule(title, desc, tags)
}
