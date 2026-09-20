package weworkremotely

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

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
	tags := skillsTags(item.Skills, item.Category)
	postedAt, _ := parseRSSPubDate(strings.TrimSpace(item.PubDate))
	countryCode := countryFromWWR(countries, item)
	j := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         listingURL,
		Description: descPlain,
		PostedAt:    postedAt,
		Remote:      remoteFromWWR(item.Region, title, descPlain, tags),
		CountryCode: countryCode,
		Tags:        tags,
		Position:    utils.InferPosition(title, descPlain, tags),
	}
	if err := domainutils.AssignStableID(&j); err != nil {
		return schema.Job{}, false, fmt.Errorf("stable id: %w", err)
	}
	return j, true, nil
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

func countryFromWWR(r *utils.CountryResolver, item rssItem) string {
	if r == nil {
		return ""
	}
	for _, candidate := range []string{item.Country, item.State, item.Region} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if code := r.Alpha2ForName(candidate); code != "" {
			return code
		}
	}
	return ""
}
