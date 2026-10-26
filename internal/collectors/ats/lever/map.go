package lever

import (
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	domainschema "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

// Wire types keep board structured fields even though facts mapping is out of scope.
type jobWire struct {
	ID               string         `json:"id"`
	Text             string         `json:"text"`
	Categories       categoriesWire `json:"categories"`
	Country          string         `json:"country"`
	CreatedAt        int64          `json:"createdAt"`
	DescriptionPlain string         `json:"descriptionPlain"`
	Description      string         `json:"description"`
	HostedURL        string         `json:"hostedUrl"`
	ApplyURL         string         `json:"applyUrl"`
	WorkplaceType    string         `json:"workplaceType"`
}

type categoriesWire struct {
	Commitment   string   `json:"commitment"`
	Department   string   `json:"department"`
	Location     string   `json:"location"`
	Team         string   `json:"team"`
	AllLocations []string `json:"allLocations"`
}

func jobFromWire(cr *utils.CountryResolver, company string, w jobWire) (domainschema.Job, error) {
	listing := strings.TrimSpace(w.HostedURL)
	if listing == "" {
		listing = strings.TrimSpace(w.ApplyURL)
	}
	if listing == "" {
		return domainschema.Job{}, fmt.Errorf("missing hostedUrl and applyUrl")
	}
	listingURL, err := utils.CanonicalListingURL(listing)
	if err != nil {
		return domainschema.Job{}, fmt.Errorf("listing URL: %w", err)
	}
	title := strings.TrimSpace(w.Text)
	if title == "" {
		return domainschema.Job{}, fmt.Errorf("empty text for %s", listingURL)
	}
	company = strings.TrimSpace(company)

	descPlain := strings.TrimSpace(w.DescriptionPlain)
	if descPlain == "" {
		descPlain = utils.StripHTMLToPlainText(w.Description)
	}

	applyURL := ""
	if a := strings.TrimSpace(w.ApplyURL); a != "" {
		if u, err := utils.CanonicalListingURL(a); err == nil {
			applyURL = u
		}
	}

	hints := locationHints(w)
	remote := remoteFromWorkplace(w.WorkplaceType, title, descPlain, hints...)
	loc := locationFromWorkplace(cr, remote, w.WorkplaceType, w.Country, hints...)

	var tags []string
	for _, s := range []string{w.Categories.Department, w.Categories.Team, w.Categories.Commitment} {
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
		PostedAt:    parseLeverTime(w.CreatedAt),
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
	if loc := strings.TrimSpace(w.Categories.Location); loc != "" {
		hints = append(hints, loc)
	}
	for _, loc := range w.Categories.AllLocations {
		if t := strings.TrimSpace(loc); t != "" {
			hints = append(hints, t)
		}
	}
	return hints
}

func remoteFromWorkplace(workplaceType, title, desc string, hints ...string) *bool {
	switch strings.ToLower(strings.TrimSpace(workplaceType)) {
	case "remote":
		v := true
		return &v
	case "on-site", "onsite", "hybrid":
		v := false
		return &v
	}
	return utils.RemoteMVPRule(title, desc, nil, hints...)
}

func locationFromWorkplace(cr *utils.CountryResolver, remote *bool, workplaceType, country string, hints ...string) domainschema.Location {
	country = strings.ToUpper(strings.TrimSpace(country))
	var loc domainschema.Location
	if cr == nil {
		loc = utils.LocationFromParsed(remote, country, nil, nil, strings.Join(trimNonEmpty(hints), ", "), nil)
	} else {
		countries, regions, raw := utils.ParseHiringScope(cr, hints...)
		loc = utils.LocationFromParsed(remote, country, countries, regions, raw, nil)
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

func parseLeverTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
