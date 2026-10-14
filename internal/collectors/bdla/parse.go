package bdla

import (
	"bytes"
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

var (
	digitsRE  = regexp.MustCompile(`^[0-9]+$`)
	angebotRE = regexp.MustCompile(`Angebot vom:\s*(\d{2}\.\d{2}\.\d{4})`)
)

type listCard struct {
	ID  string
	URL string
}

func parseListHTML(body []byte) (int, []listCard, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("bdla: list html: %w", err)
	}
	lines := doc.Find("div.listLine")
	var cards []listCard
	seen := make(map[string]struct{})
	lines.Each(func(_ int, line *goquery.Selection) {
		line.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			card, ok := listingFromHref(href)
			if !ok {
				return
			}
			if _, dup := seen[card.ID]; dup {
				return
			}
			seen[card.ID] = struct{}{}
			cards = append(cards, card)
		})
	})
	return lines.Length(), cards, nil
}

func listingFromHref(href string) (listCard, bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil || u == nil {
		return listCard{}, false
	}
	if u.Host == "" {
		u.Scheme = "https"
		u.Host = "www.bdla.de"
	}
	host := strings.ToLower(u.Hostname())
	if host != "www.bdla.de" && host != "bdla.de" {
		return listCard{}, false
	}
	path := u.Path
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	if !strings.EqualFold(path, listPath) {
		return listCard{}, false
	}
	id := strings.TrimSpace(u.Query().Get("details"))
	if !digitsRE.MatchString(id) {
		return listCard{}, false
	}
	return listCard{ID: id, URL: detailURL(id)}, true
}

func parseDetailHTML(body []byte, listing string) (schema.Job, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return schema.Job{}, fmt.Errorf("bdla: detail %s: %w", listing, err)
	}
	title := strings.TrimSpace(doc.Find("h1").First().Text())
	if title == "" {
		return schema.Job{}, fmt.Errorf("bdla: detail %s: empty title", listing)
	}
	out := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     labelValue(doc, "Firma"),
		URL:         listing,
		ApplyURL:    applyURL(doc),
		Description: description(doc),
		PostedAt:    postedAt(doc),
		Location: schema.Location{
			Raw: labelValue(doc, "Einsatzort"),
		},
	}
	if err := domainutils.AssignStableID(&out); err != nil {
		return schema.Job{}, err
	}
	return out, nil
}

func labelValue(doc *goquery.Document, label string) string {
	var found string
	doc.Find("strong").Each(func(_ int, s *goquery.Selection) {
		if found != "" {
			return
		}
		if !strings.EqualFold(strings.TrimSpace(s.Text()), label) {
			return
		}
		row := s.ParentsFiltered("div.row").First()
		row.ChildrenFiltered("div").Each(func(_ int, col *goquery.Selection) {
			if found != "" || col.Find("strong").Length() > 0 {
				return
			}
			html, err := col.Html()
			if err != nil {
				return
			}
			found = utils.StripHTMLToPlainText(html)
		})
	})
	return found
}

func description(doc *goquery.Document) string {
	var text string
	doc.Find("h3").Each(func(_ int, h *goquery.Selection) {
		if text != "" {
			return
		}
		if !strings.EqualFold(strings.TrimSpace(h.Text()), "Stellenbeschreibung") {
			return
		}
		html, err := h.NextAllFiltered("p").First().Html()
		if err != nil {
			return
		}
		text = utils.StripHTMLToPlainText(html)
	})
	return text
}

func applyURL(doc *goquery.Document) string {
	if link := linkAfterHeading(doc, "kontakt"); link != "" {
		return link
	}
	return linkInLabel(doc, "Kontakt")
}

func linkAfterHeading(doc *goquery.Document, prefix string) string {
	var link string
	doc.Find("h3").Each(func(_ int, h *goquery.Selection) {
		if link != "" {
			return
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(h.Text())), prefix) {
			return
		}
		for sib := h.Next(); sib.Length() > 0 && !sib.Is("h3"); sib = sib.Next() {
			if u := firstHTTPLink(sib); u != "" {
				link = u
				return
			}
		}
	})
	return link
}

func linkInLabel(doc *goquery.Document, label string) string {
	var link string
	doc.Find("strong").Each(func(_ int, s *goquery.Selection) {
		if link != "" || !strings.EqualFold(strings.TrimSpace(s.Text()), label) {
			return
		}
		row := s.ParentsFiltered("div.row").First()
		row.ChildrenFiltered("div").Each(func(_ int, col *goquery.Selection) {
			if link != "" || col.Find("strong").Length() > 0 {
				return
			}
			link = firstHTTPLink(col)
		})
	})
	return link
}

func firstHTTPLink(s *goquery.Selection) string {
	var link string
	s.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		if link != "" {
			return
		}
		href, _ := a.Attr("href")
		link = httpApplyURL(href)
	})
	return link
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

func postedAt(doc *goquery.Document) time.Time {
	text := doc.Text()
	if i := strings.Index(text, "Stellenbeschreibung"); i >= 0 {
		text = text[:i]
	}
	m := angebotRE.FindStringSubmatch(text)
	if m == nil {
		return time.Time{}
	}
	t, err := time.Parse("02.01.2006", m[1])
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
