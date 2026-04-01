package debughttp

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

type runCollectorResponse struct {
	OK              bool           `json:"ok"`
	Collector       string         `json:"collector"`
	Count           int            `json:"count"`
	UpstreamFetched int            `json:"upstream_fetched,omitempty"`
	Error           string         `json:"error,omitempty"`
	Jobs            []jobDebugJSON `json:"jobs,omitempty"`
}

type jobDebugJSON struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	URL         string   `json:"url"`
	ApplyURL    string   `json:"apply_url,omitempty"`
	Description string   `json:"description,omitempty"`
	PostedAt    string   `json:"posted_at,omitempty"`
	Remote      *bool    `json:"remote"`
	CountryCode string   `json:"country_code,omitempty"`
	SalaryRaw   string   `json:"salary_raw,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Position    *string  `json:"position,omitempty"`
	UserID      *string  `json:"user_id,omitempty"`
}

func jobToDebugJSON(j domain.Job) jobDebugJSON {
	out := jobDebugJSON{
		ID:          j.ID,
		Source:      j.Source,
		Title:       j.Title,
		Company:     j.Company,
		URL:         j.URL,
		ApplyURL:    j.ApplyURL,
		Description: j.Description,
		Remote:      j.Remote,
		CountryCode: j.CountryCode,
		SalaryRaw:   j.SalaryRaw,
		Tags:        j.Tags,
		Position:    j.Position,
		UserID:      j.UserID,
	}
	if !j.PostedAt.IsZero() {
		out.PostedAt = j.PostedAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}
