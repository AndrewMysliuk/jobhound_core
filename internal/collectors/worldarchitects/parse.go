package worldarchitects

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

type listCard struct {
	Slug     string
	URL      string
	PostedAt time.Time
}

func parseListHTML(body []byte) ([]listCard, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("worldarchitects: list html: %w", err)
	}
	var cards []listCard
	index := make(map[string]int)
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		slug, ok := slugFromHref(href)
		if !ok {
			return
		}
		posted := postedOnCard(a)
		if i, seen := index[slug]; seen {
			if cards[i].PostedAt.IsZero() && !posted.IsZero() {
				cards[i].PostedAt = posted
			}
			return
		}
		listing, err := listingURL(slug)
		if err != nil {
			return
		}
		index[slug] = len(cards)
		cards = append(cards, listCard{Slug: slug, URL: listing, PostedAt: posted})
	})
	return cards, nil
}

func slugFromHref(href string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil || u == nil {
		return "", false
	}
	if u.Host != "" {
		host := strings.ToLower(u.Hostname())
		if host != "world-architects.com" && host != "www.world-architects.com" {
			return "", false
		}
	}
	path := u.Path
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	const prefix = "/en/jobs/"
	if len(path) <= len(prefix) || !strings.EqualFold(path[:len(prefix)], prefix) {
		return "", false
	}
	slug := path[len(prefix):]
	if slug == "" || strings.Contains(slug, "/") {
		return "", false
	}
	return slug, true
}

func listingURL(slug string) (string, error) {
	return utils.CanonicalListingURL(siteOrigin + listPath + "/" + slug)
}

func postedOnCard(a *goquery.Selection) time.Time {
	li := a.Closest("li")
	if li.Length() == 0 {
		return time.Time{}
	}
	raw, ok := li.Find("time[datetime]").Attr("datetime")
	if !ok {
		return time.Time{}
	}
	return parsePosted(raw)
}

func parsePosted(raw string) time.Time {
	t, err := time.Parse("1/2/06, 3:04 PM", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func parseDetailHTML(body []byte, card listCard) (schema.Job, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return schema.Job{}, fmt.Errorf("worldarchitects: detail %s: %w", card.URL, err)
	}
	title := strings.TrimSpace(doc.Find("h1").First().Text())
	if title == "" {
		return schema.Job{}, fmt.Errorf("worldarchitects: detail %s: empty title", card.URL)
	}
	city, countries := jobLocation(doc)
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     providerName(doc),
		URL:         card.URL,
		Description: description(doc),
		PostedAt:    card.PostedAt,
		Location: schema.Location{
			Raw:       city,
			Countries: countries,
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func providerName(doc *goquery.Document) string {
	h5 := doc.Find("h1").First().NextFiltered("h5")
	if h5.Length() == 0 {
		return ""
	}
	if a := h5.Find("a").First(); a.Length() > 0 {
		return strings.TrimSpace(a.Text())
	}
	return strings.TrimSpace(h5.Text())
}

func description(doc *goquery.Document) string {
	html, err := doc.Find("div.col-md-7").First().Html()
	if err != nil {
		return ""
	}
	return utils.StripHTMLToPlainText(html)
}

func jobLocation(doc *goquery.Document) (string, []string) {
	var raw string
	doc.Find("dt").Each(func(_ int, dt *goquery.Selection) {
		if raw != "" {
			return
		}
		if !strings.EqualFold(strings.TrimSpace(dt.Text()), "Job Location") {
			return
		}
		raw = strings.TrimSpace(dt.NextFiltered("dd").Text())
	})
	city, country := splitLastComma(raw)
	if code := countryCode(country); code != "" {
		return city, []string{code}
	}
	return city, nil
}

func splitLastComma(raw string) (string, string) {
	s := strings.TrimSpace(raw)
	i := strings.LastIndex(s, ",")
	if i < 0 {
		return s, ""
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
}

func countryCode(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "deutschland", "germany":
		return "DE"
	case "österreich", "oesterreich", "austria":
		return "AT"
	case "schweiz", "switzerland", "suisse", "svizzera":
		return "CH"
	}
	s := strings.TrimSpace(name)
	if len(s) == 2 {
		return strings.ToUpper(s)
	}
	return ""
}
