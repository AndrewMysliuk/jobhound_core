package debughttp

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

type runCollectorResponse struct {
	OK              bool           `json:"ok"`
	Collector       string         `json:"collector"`
	Count           int            `json:"count"`
	UpstreamFetched int            `json:"upstream_fetched,omitempty"`
	Error           string         `json:"error,omitempty"`
	Jobs            []jobDebugJSON `json:"jobs,omitempty"`
}

type locationDebugJSON struct {
	Type      string   `json:"type,omitempty"`
	Regions   []string `json:"regions,omitempty"`
	Countries []string `json:"countries,omitempty"`
	Timezone  string   `json:"timezone,omitempty"`
	Raw       string   `json:"raw,omitempty"`
}

type jobDebugJSON struct {
	ID             string            `json:"id"`
	Source         string            `json:"source"`
	Sources        []string          `json:"sources,omitempty"`
	Title          string            `json:"title"`
	Company        string            `json:"company"`
	CompanyKey     string            `json:"company_key,omitempty"`
	CompanyWebsite string            `json:"company_website,omitempty"`
	URL            string            `json:"url"`
	ApplyURL       string            `json:"apply_url,omitempty"`
	Description    string            `json:"description,omitempty"`
	PostedAt       string            `json:"posted_at,omitempty"`
	Location       locationDebugJSON `json:"location"`
	SalaryRaw      string            `json:"salary_raw,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	Position       *string           `json:"position,omitempty"`
	FirstSeenAt    string            `json:"first_seen_at,omitempty"`
	LastSeenAt     string            `json:"last_seen_at,omitempty"`
}

func jobToDebugJSON(j schema.Job) jobDebugJSON {
	out := jobDebugJSON{
		ID:             j.ID,
		Source:         j.Source,
		Sources:        j.Sources,
		Title:          j.Title,
		Company:        j.Company,
		CompanyKey:     j.CompanyKey,
		CompanyWebsite: j.CompanyWebsite,
		URL:            j.URL,
		ApplyURL:       j.ApplyURL,
		Description:    j.Description,
		Location: locationDebugJSON{
			Type:      j.Location.Type,
			Regions:   j.Location.Regions,
			Countries: j.Location.Countries,
			Timezone:  j.Location.Timezone,
			Raw:       j.Location.Raw,
		},
		SalaryRaw: j.SalaryRaw,
		Tags:      j.Tags,
		Position:  j.Position,
	}
	if !j.PostedAt.IsZero() {
		out.PostedAt = j.PostedAt.UTC().Format(time.RFC3339Nano)
	}
	if !j.FirstSeenAt.IsZero() {
		out.FirstSeenAt = j.FirstSeenAt.UTC().Format(time.RFC3339Nano)
	}
	if !j.LastSeenAt.IsZero() {
		out.LastSeenAt = j.LastSeenAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}
