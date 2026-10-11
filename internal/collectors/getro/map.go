package getro

import (
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	domainutils "github.com/andrewmysliuk/jobhound_core/internal/domain/utils"
)

type jobHit struct {
	Title        string   `json:"title"`
	URL          string   `json:"url"`
	CreatedAt    *int64   `json:"created_at"`
	Locations    []string `json:"locations"`
	WorkMode     string   `json:"work_mode"`
	Skills       []string `json:"skills"`
	Organization struct {
		Name string `json:"name"`
	} `json:"organization"`
}

func mapHit(sourceID string, countries *utils.CountryResolver, hit jobHit) (schema.Job, bool) {
	title := strings.TrimSpace(hit.Title)
	company := strings.TrimSpace(hit.Organization.Name)
	rawURL := strings.TrimSpace(hit.URL)
	if title == "" || company == "" || rawURL == "" {
		return schema.Job{}, false
	}
	labels := skillLabels(hit.Skills)
	job := schema.Job{
		Source:      sourceID,
		Title:       title,
		Company:     company,
		URL:         rawURL,
		ApplyURL:    rawURL,
		Description: strings.Join(labels, "\n"),
		Tags:        labels,
		PostedAt:    postedAtFromUnix(hit.CreatedAt),
		Location:    locationFromHit(countries, hit),
	}
	if err := domainutils.AssignStableID(&job); err != nil {
		return schema.Job{}, false
	}
	return job, true
}

func skillLabels(skills []string) []string {
	if len(skills) == 0 {
		return nil
	}
	labels := make([]string, 0, len(skills))
	for _, skill := range skills {
		skill = strings.TrimSpace(skill)
		if skill == "" {
			continue
		}
		labels = append(labels, skill)
	}
	return labels
}

func postedAtFromUnix(sec *int64) time.Time {
	if sec == nil || *sec <= 0 {
		return time.Time{}
	}
	return time.Unix(*sec, 0).UTC()
}

func locationFromHit(countries *utils.CountryResolver, hit jobHit) schema.Location {
	parts := make([]string, 0, len(hit.Locations))
	for _, loc := range hit.Locations {
		loc = strings.TrimSpace(loc)
		if loc == "" {
			continue
		}
		parts = append(parts, loc)
	}
	countriesOut, regions, _ := utils.ParseHiringScope(countries, parts...)
	loc := schema.Location{
		Countries: countriesOut,
		Regions:   regions,
		Raw:       strings.Join(parts, ", "),
	}
	if strings.TrimSpace(hit.WorkMode) == "remote" {
		loc.Type = schema.LocationRemote
	}
	return loc
}
