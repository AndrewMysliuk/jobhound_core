package europeremotely

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

var (
	relativePostedRE = regexp.MustCompile(`(?i)posted\s+(.+)`)
	agoRE            = regexp.MustCompile(`(?i)^\s*(\d+)\s+(second|minute|hour|day|week|month|year)s?\s+ago\s*$`)
)

type listingCard struct {
	title         string
	company       string
	locationRaw   string
	postedDisplay string
	compensation  string
	jobPageURL    string
}

func parseListingCards(htmlFragment string, base *url.URL) ([]listingCard, error) {
	wrapped := "<html><body>" + htmlFragment + "</body></html>"
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return nil, fmt.Errorf("listing fragment: %w", err)
	}
	var out []listingCard
	var parseErr error
	doc.Find(selListingRoot).Each(func(_ int, s *goquery.Selection) {
		if parseErr != nil {
			return
		}
		card, err := parseOneListingCard(s, base)
		if err != nil {
			parseErr = err
			return
		}
		if strings.TrimSpace(card.jobPageURL) == "" {
			parseErr = fmt.Errorf("listing card: missing job page URL")
			return
		}
		out = append(out, card)
	})
	if parseErr != nil {
		return nil, parseErr
	}
	return out, nil
}

func parseOneListingCard(s *goquery.Selection, base *url.URL) (listingCard, error) {
	href, _ := s.Find(selListingTitleLink).First().Attr("href")
	if strings.TrimSpace(href) == "" {
		if wrap := s.Closest(selListingCardWrap); wrap.Length() > 0 {
			href, _ = wrap.First().Attr("href")
		}
	}
	abs, err := absoluteURL(base, strings.TrimSpace(href))
	if err != nil {
		return listingCard{}, fmt.Errorf("job link: %w", err)
	}
	card := listingCard{
		title:         strings.TrimSpace(s.Find(selListingTitle).First().Text()),
		company:       strings.TrimSpace(s.Find(selListingCompany).First().Text()),
		locationRaw:   strings.TrimSpace(s.Find(selListingLocation).First().Text()),
		postedDisplay: strings.TrimSpace(s.Find(selListingPosted).First().Text()),
		jobPageURL:    abs,
	}
	s.Find(selListingMetaItem).Each(func(_ int, m *goquery.Selection) {
		class, _ := m.Attr("class")
		if strings.Contains(class, "meta-location") || strings.Contains(class, "meta-type") ||
			strings.Contains(class, "meta-level") || strings.Contains(class, "meta-category") {
			return
		}
		if strings.TrimSpace(m.Text()) != "" && card.compensation == "" {
			card.compensation = strings.TrimSpace(m.Text())
		}
	})
	return card, nil
}

type detailParsed struct {
	title           string
	company         string
	locationRaw     string
	postedDisplay   string
	compensationRaw string
	description     string
	tags            []string
	applyURL        string
}

func parseJobDetailHTML(html string, base *url.URL) (detailParsed, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return detailParsed{}, fmt.Errorf("detail document: %w", err)
	}
	d := detailParsed{
		title: strings.TrimSpace(doc.Find(selDetailTitle).First().Text()),
	}
	d.company = strings.TrimSpace(doc.Find(selDetailCompany).First().Text())
	loc := doc.Find(selDetailLocLink).First()
	if loc.Length() > 0 {
		d.locationRaw = strings.TrimSpace(loc.Text())
	} else {
		d.locationRaw = strings.TrimSpace(doc.Find(selDetailLocation).First().Text())
	}
	d.postedDisplay = strings.TrimSpace(doc.Find(selDetailDatePosted).First().Text())
	d.compensationRaw = strings.TrimSpace(doc.Find(selDetailSalary).First().Text())
	descSel := doc.Find(selDetailDescription).First()
	d.description = utils.NormalizePlainText(descSel.Text())
	if href, ok := doc.Find(selDetailApply).First().Attr("href"); ok {
		au, err := absoluteURL(base, strings.TrimSpace(href))
		if err == nil {
			d.applyURL = au
		}
	}
	tagsRaw := strings.TrimSpace(doc.Find(selDetailTags).First().Text())
	d.tags = parseJobTagsLine(tagsRaw)
	return d, nil
}

func parseJobTagsLine(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if idx := strings.Index(strings.ToLower(s), "tagged as:"); idx >= 0 {
		s = strings.TrimSpace(s[idx+len("tagged as:"):])
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func absoluteURL(base *url.URL, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty href")
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	if u.IsAbs() {
		return u.String(), nil
	}
	if base == nil {
		return "", fmt.Errorf("relative href %q without base URL", ref)
	}
	return base.ResolveReference(u).String(), nil
}

func resolvePostedAt(now time.Time, listingPosted, detailPosted string, warn func(string)) time.Time {
	detailPosted = strings.TrimSpace(detailPosted)
	listingPosted = strings.TrimSpace(listingPosted)
	if detailPosted != "" {
		if t, ok := parseAbsoluteDate(detailPosted); ok {
			return t.UTC()
		}
		if t, ok := parseRelativePosted(now, detailPosted); ok {
			return t.UTC()
		}
		if warn != nil {
			warn(detailPosted)
		}
	}
	if listingPosted != "" {
		if t, ok := parseRelativePosted(now, listingPosted); ok {
			return t.UTC()
		}
		if warn != nil {
			warn(listingPosted)
		}
	}
	return time.Time{}
}

func parseAbsoluteDate(s string) (time.Time, bool) {
	layouts := []string{
		"January 2, 2006",
		"January 02, 2006",
		"2 January 2006",
		"02 January 2006",
		"2006-01-02",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseRelativePosted(now time.Time, display string) (time.Time, bool) {
	display = strings.TrimSpace(display)
	m := relativePostedRE.FindStringSubmatch(display)
	if len(m) == 2 {
		display = strings.TrimSpace(m[1])
	}
	am := agoRE.FindStringSubmatch(strings.TrimSpace(display))
	if len(am) != 3 {
		return time.Time{}, false
	}
	var n int
	fmt.Sscanf(am[1], "%d", &n)
	if n <= 0 {
		return time.Time{}, false
	}
	unit := strings.ToLower(am[2])
	now = now.UTC()
	var d time.Duration
	switch unit {
	case "second":
		d = time.Duration(n) * time.Second
	case "minute":
		d = time.Duration(n) * time.Minute
	case "hour":
		d = time.Duration(n) * time.Hour
	case "day":
		d = time.Duration(n) * 24 * time.Hour
	case "week":
		d = time.Duration(n) * 7 * 24 * time.Hour
	case "month":
		d = time.Duration(n) * 30 * 24 * time.Hour
	case "year":
		d = time.Duration(n) * 365 * 24 * time.Hour
	default:
		return time.Time{}, false
	}
	return now.Add(-d), true
}

func salaryRaw(listingComp, detailComp string) string {
	listingComp = strings.TrimSpace(listingComp)
	detailComp = strings.TrimSpace(detailComp)
	switch {
	case listingComp != "" && detailComp != "" && listingComp != detailComp:
		return listingComp + " | " + detailComp
	case detailComp != "":
		return detailComp
	default:
		return listingComp
	}
}
