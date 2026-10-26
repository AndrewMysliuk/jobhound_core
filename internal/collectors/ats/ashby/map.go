package ashby

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
	Jobs       []jobWire `json:"jobs"`
	APIVersion string    `json:"apiVersion"`
}

type jobWire struct {
	ID                 string         `json:"id"`
	Title              string         `json:"title"`
	Department         string         `json:"department"`
	Team               string         `json:"team"`
	EmploymentType     string         `json:"employmentType"`
	Location           string         `json:"location"`
	SecondaryLocations []secondaryLoc `json:"secondaryLocations"`
	PublishedAt        string         `json:"publishedAt"`
	IsListed           bool           `json:"isListed"`
	IsRemote           bool           `json:"isRemote"`
	WorkplaceType      string         `json:"workplaceType"`
	Address            *addressWire   `json:"address"`
	JobURL             string         `json:"jobUrl"`
	ApplyURL           string         `json:"applyUrl"`
	DescriptionHTML    string         `json:"descriptionHtml"`
	DescriptionPlain   string         `json:"descriptionPlain"`
}

type secondaryLoc struct {
	Location string       `json:"location"`
	Address  *addressWire `json:"address"`
}

type addressWire struct {
	PostalAddress *postalAddress `json:"postalAddress"`
}

type postalAddress struct {
	AddressCountry  string `json:"addressCountry"`
	AddressRegion   string `json:"addressRegion"`
	AddressLocality string `json:"addressLocality"`
}

func jobFromWire(cr *utils.CountryResolver, company string, w jobWire) (domainschema.Job, error) {
	listing := strings.TrimSpace(w.JobURL)
	if listing == "" {
		listing = strings.TrimSpace(w.ApplyURL)
	}
	if listing == "" {
		return domainschema.Job{}, fmt.Errorf("missing jobUrl and applyUrl")
	}
	listingURL, err := utils.CanonicalListingURL(listing)
	if err != nil {
		return domainschema.Job{}, fmt.Errorf("listing URL: %w", err)
	}
	title := strings.TrimSpace(w.Title)
	if title == "" {
		return domainschema.Job{}, fmt.Errorf("empty title for %s", listingURL)
	}
	company = strings.TrimSpace(company)

	descPlain := strings.TrimSpace(w.DescriptionPlain)
	if descPlain == "" {
		descPlain = utils.StripHTMLToPlainText(w.DescriptionHTML)
	}

	applyURL := ""
	if a := strings.TrimSpace(w.ApplyURL); a != "" {
		if u, err := utils.CanonicalListingURL(a); err == nil {
			applyURL = u
		}
	}

	hints := locationHints(w)
	remote := remoteFromWorkplace(w.IsRemote, w.WorkplaceType, title, descPlain, hints...)
	loc := locationFromWorkplace(cr, remote, w.WorkplaceType, hints...)

	var tags []string
	for _, s := range []string{w.Department, w.Team, w.EmploymentType} {
		if t := strings.TrimSpace(s); t != "" {
			tags = append(tags, t)
		}
	}

	j := domainschema.Job{
		Source:      SourceName,
		Title:       title,
		Company:     company,
		URL:         listingURL,
		ApplyURL:    applyURL,
		Description: descPlain,
		PostedAt:    parseAshbyTime(w.PublishedAt),
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
	if loc := strings.TrimSpace(w.Location); loc != "" {
		hints = append(hints, loc)
	}
	if w.Address != nil && w.Address.PostalAddress != nil {
		pa := w.Address.PostalAddress
		for _, s := range []string{pa.AddressLocality, pa.AddressRegion, pa.AddressCountry} {
			if t := strings.TrimSpace(s); t != "" {
				hints = append(hints, t)
			}
		}
	}
	for _, sec := range w.SecondaryLocations {
		if sec.Address != nil && sec.Address.PostalAddress != nil {
			if c := strings.TrimSpace(sec.Address.PostalAddress.AddressCountry); c != "" {
				hints = append(hints, c)
				continue
			}
		}
		if loc := strings.TrimSpace(sec.Location); loc != "" {
			hints = append(hints, loc)
		}
	}
	return hints
}

func remoteFromWorkplace(isRemote bool, workplaceType, title, desc string, hints ...string) *bool {
	switch strings.ToLower(strings.TrimSpace(workplaceType)) {
	case "remote":
		v := true
		return &v
	case "on-site", "onsite", "hybrid":
		v := false
		return &v
	}
	if isRemote {
		v := true
		return &v
	}
	return utils.RemoteMVPRule(title, desc, nil, hints...)
}

func locationFromWorkplace(cr *utils.CountryResolver, remote *bool, workplaceType string, hints ...string) domainschema.Location {
	var loc domainschema.Location
	if cr == nil {
		loc = utils.LocationFromParsed(remote, "", nil, nil, strings.Join(trimNonEmpty(hints), ", "), nil)
	} else {
		countries, regions, raw := utils.ParseHiringScope(cr, hints...)
		loc = utils.LocationFromParsed(remote, "", countries, regions, raw, nil)
	}
	switch strings.ToLower(strings.TrimSpace(workplaceType)) {
	case "remote":
		loc.Type = domainschema.LocationRemote
	case "hybrid":
		loc.Type = domainschema.LocationHybrid
	case "on-site", "onsite":
		loc.Type = domainschema.LocationOffice
	}
	return loc
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

func parseAshbyTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
