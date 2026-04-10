package djinni

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var jobPathRE = regexp.MustCompile(`^/jobs/(\d+)-[^/]+/?$`)

type listingCard struct {
	jobURL        string
	title         string
	company       string
	salaryPreview string
	metaHints     []string
	tags          []string
}

func parseListingCards(html string, base *url.URL) ([]listingCard, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("listing document: %w", err)
	}
	var out []listingCard
	var parseErr error
	doc.Find("h2.job-item__position").Each(func(_ int, h2 *goquery.Selection) {
		if parseErr != nil {
			return
		}
		link := h2.Parent()
		if !link.Is("a") {
			link = h2.ParentsFiltered("a").First()
		}
		if link.Length() == 0 {
			parseErr = fmt.Errorf("listing: job title without link")
			return
		}
		href, _ := link.Attr("href")
		href = strings.TrimSpace(href)
		if href == "" {
			parseErr = fmt.Errorf("listing: empty job href")
			return
		}
		abs, err := absoluteURL(base, href)
		if err != nil {
			parseErr = fmt.Errorf("listing job link: %w", err)
			return
		}
		u, err := url.Parse(abs)
		if err != nil || !jobPathRE.MatchString(u.Path) {
			return
		}
		root := link.Closest("li, article, div")
		if root.Length() == 0 {
			root = link.Parent()
		}
		company := strings.TrimSpace(root.Find("span.small.text-gray-800.opacity-75.font-weight-500").First().Text())
		salaryPreview := strings.TrimSpace(root.Find("div.col-auto div.fs-5 strong.text-success").First().Text())
		var hints []string
		root.Find("div.fw-medium").First().Find("span").Each(func(_ int, sp *goquery.Selection) {
			t := strings.TrimSpace(sp.Text())
			if t != "" {
				hints = append(hints, t)
			}
		})
		var tags []string
		root.Find("div.job-item__tags span.badge").Each(func(_ int, b *goquery.Selection) {
			t := strings.TrimSpace(b.Text())
			if t != "" {
				tags = append(tags, t)
			}
		})
		title := strings.TrimSpace(h2.Text())
		out = append(out, listingCard{
			jobURL:        abs,
			title:         title,
			company:       company,
			salaryPreview: salaryPreview,
			metaHints:     hints,
			tags:          tags,
		})
	})
	if parseErr != nil {
		return nil, parseErr
	}
	return dedupeListingCards(out), nil
}

func dedupeListingCards(in []listingCard) []listingCard {
	seen := make(map[string]struct{})
	var out []listingCard
	for _, c := range in {
		key := strings.TrimSpace(c.jobURL)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
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

func buildListingURL(base *url.URL, keywords string, page int) string {
	u := *base
	u.Path = "/jobs/"
	q := url.Values{}
	q.Set("all_keywords", keywords)
	q.Set("search_type", "full-text")
	q.Set("page", fmt.Sprintf("%d", page))
	u.RawQuery = q.Encode()
	return u.String()
}
