package dou

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

type listingCard struct {
	title         string
	company       string
	locationRaw   string
	postedDisplay string
	jobPageURL    string
}

func parseListingCards(htmlFragment string, base *url.URL) ([]listingCard, error) {
	wrapped := "<html><body><ul>" + htmlFragment + "</ul></body></html>"
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

func parseListingCardsFromDocument(doc *goquery.Document, base *url.URL) ([]listingCard, error) {
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
	href, _ := s.Find(selListingTitle).First().Attr("href")
	abs, err := absoluteURL(base, strings.TrimSpace(href))
	if err != nil {
		return listingCard{}, fmt.Errorf("job link: %w", err)
	}
	return listingCard{
		title:         strings.TrimSpace(s.Find(selListingTitle).First().Text()),
		company:       strings.TrimSpace(s.Find(selListingCompany).First().Text()),
		locationRaw:   strings.TrimSpace(s.Find(selListingCities).First().Text()),
		postedDisplay: strings.TrimSpace(s.Find(selListingDate).First().Text()),
		jobPageURL:    abs,
	}, nil
}

type detailParsed struct {
	title         string
	postedDisplay string
	locationRaw   string
	salaryRaw     string
	description   string
	tags          []string
}

func parseJobDetailHTML(html string, base *url.URL) (detailParsed, error) {
	_ = base // reserved for future apply-link resolution
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return detailParsed{}, fmt.Errorf("detail document: %w", err)
	}
	d := detailParsed{
		title:         strings.TrimSpace(doc.Find(selDetailTitle).First().Text()),
		postedDisplay: strings.TrimSpace(doc.Find(selDetailDate).First().Text()),
		locationRaw:   strings.TrimSpace(doc.Find(selDetailPlace).First().Text()),
		salaryRaw:     strings.TrimSpace(doc.Find(selDetailSalary).First().Text()),
	}
	descSel := doc.Find(selDetailBody).First()
	d.description = utils.NormalizePlainText(descSel.Text())
	var tags []string
	doc.Find(selDetailBadge).Each(func(_ int, b *goquery.Selection) {
		t := strings.TrimSpace(b.Text())
		if t != "" {
			tags = append(tags, t)
		}
	})
	d.tags = tags
	return d, nil
}

func extractCSRFToken(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	v, _ := doc.Find(`input[name="csrfmiddlewaretoken"]`).First().Attr("value")
	return strings.TrimSpace(v)
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

var ukrainianDateRE = regexp.MustCompile(`^\s*(\d{1,2})\s+([^\d\s]+?)(?:\s+(\d{4}))?\s*$`)

var ukrainianMonths = map[string]time.Month{
	"січня":     time.January,
	"лютого":    time.February,
	"березня":   time.March,
	"квітня":    time.April,
	"травня":    time.May,
	"червня":    time.June,
	"липня":     time.July,
	"серпня":    time.August,
	"вересня":   time.September,
	"жовтня":    time.October,
	"листопада": time.November,
	"грудня":    time.December,
}

func normalizeMonthKey(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(strings.ToLower(s)) {
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// parseUkrainianPostedDisplay parses DOU human dates (domain-mapping-mvp.md).
func parseUkrainianPostedDisplay(display string, anchor time.Time) (time.Time, bool) {
	display = strings.TrimSpace(display)
	if display == "" {
		return time.Time{}, false
	}
	m := ukrainianDateRE.FindStringSubmatch(display)
	if len(m) != 4 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(m[1])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}
	monthKey := normalizeMonthKey(m[2])
	month, ok := ukrainianMonths[monthKey]
	if !ok {
		return time.Time{}, false
	}
	year := anchor.Year()
	if m[3] != "" {
		year, err = strconv.Atoi(m[3])
		if err != nil {
			return time.Time{}, false
		}
	}
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if m[3] == "" && t.After(anchor.Add(24*time.Hour)) {
		t = t.AddDate(-1, 0, 0)
	}
	return t, true
}

func resolvePostedAt(now time.Time, listingPosted, detailPosted string, warn func(string)) time.Time {
	detailPosted = strings.TrimSpace(detailPosted)
	listingPosted = strings.TrimSpace(listingPosted)
	if detailPosted != "" {
		if t, ok := parseUkrainianPostedDisplay(detailPosted, now); ok {
			return t.UTC()
		}
		if warn != nil {
			warn(detailPosted)
		}
	}
	if listingPosted != "" {
		if t, ok := parseUkrainianPostedDisplay(listingPosted, now); ok {
			return t.UTC()
		}
		if warn != nil {
			warn(listingPosted)
		}
	}
	return time.Time{}
}
