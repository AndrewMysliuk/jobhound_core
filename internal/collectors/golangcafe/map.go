package golangcafe

import (
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

func jobsFromPosts(posts []jobPost, maxJobs int) ([]schema.Job, error) {
	var out []schema.Job
	seen := make(map[string]struct{})
	for _, p := range posts {
		if maxJobs > 0 && len(out) >= maxJobs {
			break
		}
		if !matchesEuropeRemoteCatalog(p) {
			continue
		}
		j, ok, err := jobFromPost(p)
		if err != nil || !ok {
			continue
		}
		if _, dup := seen[j.URL]; dup {
			continue
		}
		seen[j.URL] = struct{}{}
		out = append(out, j)
	}
	return out, nil
}

func jobFromPost(p jobPost) (schema.Job, bool, error) {
	title := strings.TrimSpace(p.Title)
	company := strings.TrimSpace(p.Company)
	id := strings.TrimSpace(p.ID)
	if title == "" || company == "" || id == "" {
		return schema.Job{}, false, nil
	}

	cardRaw := JobCardURL(company, title, id)
	listingURL, err := utils.CanonicalListingURL(cardRaw)
	if err != nil {
		return schema.Job{}, false, err
	}

	var applyURL string
	if link := strings.TrimSpace(p.Link); link != "" {
		if u, err := utils.CanonicalListingURL(link); err == nil {
			applyURL = u
		} else if strings.HasPrefix(strings.ToLower(link), "mailto:") {
			applyURL = link
		}
	}

	descPlain := utils.StripHTMLToPlainText(p.Description)
	tags := normalizeTags(p.Tags)
	postedAt := postedAtFromEpochMs(p.Date)

	j := schema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         listingURL,
		ApplyURL:    applyURL,
		Description: descPlain,
		PostedAt:    postedAt,
		Remote:      remoteFromPost(p),
		CountryCode: countryCodeFromPost(p),
		SalaryRaw:   formatSalaryRaw(p),
		Tags:        tags,
		Position:    utils.InferPosition(title, descPlain, tags),
	}
	if err := domainutils.AssignStableID(&j); err != nil {
		return schema.Job{}, false, err
	}
	return j, true, nil
}

func matchesEuropeRemoteCatalog(p jobPost) bool {
	r := strings.ToLower(strings.TrimSpace(p.Remote))
	if r == "remote" || r == "partially_remote" {
		return true
	}
	c := strings.ToUpper(strings.TrimSpace(p.Country))
	if c == "EU" {
		return true
	}
	switch c {
	case "DE", "UA", "NL", "GB", "UK", "FR", "PL", "ES", "IT", "SE", "NO", "DK", "FI", "IE", "BE", "AT", "CH", "CZ", "RO", "PT", "GR", "HU":
		return true
	}
	if strings.EqualFold(strings.TrimSpace(p.Location), "Remote") {
		return true
	}
	return false
}

func remoteFromPost(p jobPost) *bool {
	switch strings.ToLower(strings.TrimSpace(p.Remote)) {
	case "remote", "partially_remote":
		v := true
		return &v
	case "on_site":
		v := false
		return &v
	default:
		return utils.RemoteMVPRule(p.Title, p.Description, normalizeTags(p.Tags))
	}
}

func countryCodeFromPost(p jobPost) string {
	c := strings.ToUpper(strings.TrimSpace(p.Country))
	if c == "EU" {
		return ""
	}
	if c == "UK" {
		return "GB"
	}
	if len(c) == 2 {
		return c
	}
	return ""
}

func postedAtFromEpochMs(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func normalizeTags(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	var out []string
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func formatSalaryRaw(p jobPost) string {
	cur := strings.TrimSpace(p.Currency)
	from := p.SalaryFrom.n
	to := p.SalaryTo.n
	if from == nil && to == nil {
		return ""
	}
	prefix := cur
	if prefix != "" {
		prefix += " "
	}
	switch {
	case from != nil && to != nil:
		return fmt.Sprintf("%s%.0f–%.0f", prefix, *from, *to)
	case from != nil:
		return fmt.Sprintf("%s%.0f", prefix, *from)
	default:
		return fmt.Sprintf("%s%.0f", prefix, *to)
	}
}
