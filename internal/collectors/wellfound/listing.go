package wellfound

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var jobListingPathRE = regexp.MustCompile(`^/jobs/\d+-[a-z0-9-]+$`)

// ListingJob is a row parsed from a role listing page (no description).
type ListingJob struct {
	Title   string
	Company string
	Path    string // e.g. /jobs/2416798-frontend-engineer
}

// ParseListingHTML extracts job title, company, and /jobs/{id}-{slug} href from listing HTML.
func ParseListingHTML(html string) ([]ListingJob, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("listing document: %w", err)
	}
	var out []ListingJob
	doc.Find(`div[data-testid="startup-header"]`).Each(func(_ int, header *goquery.Selection) {
		company := strings.TrimSpace(header.Find("h2").First().Text())
		card := header.Closest("div.mb-6")
		if card.Length() == 0 {
			card = header.Parent().Parent()
		}
		card.Find(`a[href^="/jobs/"]`).Each(func(_ int, a *goquery.Selection) {
			path, _ := a.Attr("href")
			path = strings.TrimSpace(path)
			if path == "" || !jobListingPathRE.MatchString(path) {
				return
			}
			title := strings.TrimSpace(a.Text())
			if title == "" || company == "" {
				return
			}
			out = append(out, ListingJob{
				Title:   title,
				Company: company,
				Path:    path,
			})
		})
	})
	return out, nil
}
