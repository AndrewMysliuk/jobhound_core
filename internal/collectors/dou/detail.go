package dou

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

const douSiteOrigin = "https://dou.ua"

// detailFields is the HTML vacancy parse result.
type detailFields struct {
	Title       string
	Company     string
	Description string
	PlaceRaw    string    // → Job.Location.Raw
	PostedAt    time.Time // zero if unparsed
	ApplyURL    string    // external goto only; empty for on-DOU #apply
}

// Ukrainian genitive month names as used in vacancy .date (e.g. "21 жовтня 2026").
var uaMonths = map[string]time.Month{
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

func fetchDetailBytes(ctx context.Context, client *http.Client, detailURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, detailURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	utils.SetCollectorUserAgent(req)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(snippet))
	}
	return body, nil
}

func parseDetailHTML(body []byte) (detailFields, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return detailFields{}, fmt.Errorf("parse HTML: %w", err)
	}

	title := utils.NormalizePlainText(doc.Find("h1").First().Text())
	if title == "" {
		title = utils.NormalizePlainText(doc.Find(".g-h2").First().Text())
	}
	company := utils.NormalizePlainText(doc.Find(".b-compinfo .l-n a").First().Text())

	descHTML, err := doc.Find(".b-typo").First().Html()
	if err != nil || strings.TrimSpace(descHTML) == "" {
		descHTML, err = doc.Find(".vacancy-section").First().Html()
		if err != nil {
			descHTML = ""
		}
	}
	description := utils.StripHTMLToPlainText(descHTML)
	if description == "" {
		description = utils.NormalizePlainText(doc.Find(".b-typo").First().Text())
	}
	if description == "" {
		description = utils.NormalizePlainText(doc.Find(".vacancy-section").First().Text())
	}

	placeRaw := utils.NormalizePlainText(doc.Find(".place").First().Text())
	dateRaw := utils.NormalizePlainText(doc.Find(".date").First().Text())

	applyHref, _ := doc.Find("a.replied-external").First().Attr("href")
	if applyHref == "" {
		// On-DOU form only — leave ApplyURL empty.
		if href, ok := doc.Find(`a[href="#apply"]`).First().Attr("href"); ok {
			applyHref = href
		}
	}

	return newDetailFields(title, company, description, placeRaw, dateRaw, applyHref), nil
}

func newDetailFields(title, company, description, placeRaw, dateRaw, applyHref string) detailFields {
	return detailFields{
		Title:       strings.TrimSpace(title),
		Company:     strings.TrimSpace(company),
		Description: strings.TrimSpace(description),
		PlaceRaw:    strings.TrimSpace(placeRaw),
		PostedAt:    parseDouDate(dateRaw),
		ApplyURL:    applyURLFromHref(applyHref),
	}
}

func parseDouDate(raw string) time.Time {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}
	}
	parts := strings.Fields(s)
	if len(parts) < 3 {
		return time.Time{}
	}
	day, err := strconv.Atoi(parts[0])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}
	}
	month, ok := uaMonths[strings.ToLower(parts[1])]
	if !ok {
		return time.Time{}
	}
	year, err := strconv.Atoi(parts[2])
	if err != nil || year < 1970 {
		return time.Time{}
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// applyURLFromHref keeps external dou.ua/goto/vacancy links; #apply and other fragments → empty.
func applyURLFromHref(href string) string {
	h := strings.TrimSpace(href)
	if h == "" || h == "#apply" || strings.HasPrefix(h, "#") {
		return ""
	}
	u, err := url.Parse(h)
	if err != nil {
		return ""
	}
	if !u.IsAbs() {
		base, err := url.Parse(douSiteOrigin + "/")
		if err != nil {
			return ""
		}
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	if strings.TrimSuffix(u.Path, "/") != "/goto/vacancy" {
		return ""
	}
	id := strings.TrimSpace(u.Query().Get("id"))
	if id == "" {
		return ""
	}
	return "https://dou.ua/goto/vacancy/?id=" + url.QueryEscape(id)
}
