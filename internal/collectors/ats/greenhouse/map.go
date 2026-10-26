package greenhouse

import (
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	domainschema "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// Wire types keep board structured fields even though facts mapping is out of scope.
type boardEnvelope struct {
	Jobs []jobWire `json:"jobs"`
	Meta metaWire  `json:"meta"`
}

type metaWire struct {
	Total int `json:"total"`
}

type jobWire struct {
	ID             int64            `json:"id"`
	Title          string           `json:"title"`
	CompanyName    string           `json:"company_name"`
	AbsoluteURL    string           `json:"absolute_url"`
	Content        string           `json:"content"`
	UpdatedAt      string           `json:"updated_at"`
	FirstPublished string           `json:"first_published"`
	Location       locationWire     `json:"location"`
	Offices        []officeWire     `json:"offices"`
	Departments    []departmentWire `json:"departments"`
}

type locationWire struct {
	Name string `json:"name"`
}

type officeWire struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

type departmentWire struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func jobFromWire(cr *utils.CountryResolver, companyFallback string, w jobWire) (domainschema.Job, error) {
	listing := strings.TrimSpace(w.AbsoluteURL)
	if listing == "" {
		return domainschema.Job{}, fmt.Errorf("missing absolute_url")
	}
	listingURL, err := utils.CanonicalListingURL(listing)
	if err != nil {
		return domainschema.Job{}, fmt.Errorf("listing URL: %w", err)
	}
	title := strings.TrimSpace(w.Title)
	if title == "" {
		return domainschema.Job{}, fmt.Errorf("empty title for %s", listingURL)
	}
	company := strings.TrimSpace(w.CompanyName)
	if company == "" {
		company = strings.TrimSpace(companyFallback)
	}

	descPlain := utils.StripHTMLToPlainText(w.Content)
	hints := locationHints(w)
	remote := remoteFromHints(title, descPlain, hints...)
	loc := locationFromHints(cr, remote, hints...)

	var tags []string
	for _, d := range w.Departments {
		if n := strings.TrimSpace(d.Name); n != "" {
			tags = append(tags, n)
		}
	}

	j := domainschema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         listingURL,
		ApplyURL:    listingURL,
		Description: descPlain,
		PostedAt:    parseGreenhouseTime(w.FirstPublished, w.UpdatedAt),
		Location:    loc,
		Tags:        tags,
		Position:    utils.InferPosition(title, descPlain, tags),
	}
	if err := domainutils.AssignStableID(&j); err != nil {
		return domainschema.Job{}, err
	}
	return j, nil
}

func locationHints(w jobWire) []string {
	var hints []string
	if n := strings.TrimSpace(w.Location.Name); n != "" {
		hints = append(hints, n)
	}
	for _, o := range w.Offices {
		if loc := strings.TrimSpace(o.Location); loc != "" {
			hints = append(hints, loc)
			continue
		}
		if n := strings.TrimSpace(o.Name); n != "" {
			hints = append(hints, n)
		}
	}
	return hints
}

func remoteFromHints(title, desc string, hints ...string) *bool {
	for _, h := range hints {
		if strings.EqualFold(strings.TrimSpace(h), "remote") {
			v := true
			return &v
		}
	}
	return utils.RemoteMVPRule(title, desc, nil, hints...)
}

func locationFromHints(cr *utils.CountryResolver, remote *bool, hints ...string) domainschema.Location {
	if cr == nil {
		return utils.LocationFromParsed(remote, "", nil, nil, strings.Join(trimNonEmpty(hints), ", "), nil)
	}
	countries, regions, raw := utils.ParseHiringScope(cr, hints...)
	return utils.LocationFromParsed(remote, "", countries, regions, raw, nil)
}

func trimNonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseGreenhouseTime(firstPublished, updatedAt string) time.Time {
	for _, raw := range []string{firstPublished, updatedAt} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
