package storage

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// Job is the GORM model for the jobs table.
type Job struct {
	ID             string     `gorm:"column:id;primaryKey;type:text"`
	Sources        textArray  `gorm:"column:sources;type:text[];not null;default:'{}'"`
	Title          string     `gorm:"column:title;type:text;not null;default:''"`
	Company        string     `gorm:"column:company;type:text;not null;default:''"`
	CompanyKey     string     `gorm:"column:company_key;type:text;not null;default:''"`
	CompanyKeyRef  *string    `gorm:"column:company_key_ref;type:text"`
	CompanyWebsite string     `gorm:"column:company_website;type:text;not null;default:''"`
	URL            string     `gorm:"column:url;type:text;not null;default:''"`
	ApplyURL       *string    `gorm:"column:apply_url;type:text"`
	Description    string     `gorm:"column:description;type:text;not null;default:''"`
	PostedAt       *time.Time `gorm:"column:posted_at"`
	Location       []byte     `gorm:"column:location;type:jsonb;not null"`
	Openings       []byte     `gorm:"column:openings;type:jsonb;not null"`
	SalaryRaw      string     `gorm:"column:salary_raw;type:text;not null;default:''"`
	Tags           []byte     `gorm:"column:tags;type:jsonb;not null"`
	Position       *string    `gorm:"column:position;type:text"`
	ClosedAt       *time.Time `gorm:"column:closed_at"`
	ClosedBy       string     `gorm:"column:closed_by;type:text;not null;default:''"`
	BoardSeenAt    *time.Time `gorm:"column:board_seen_at"`
	FirstSeenAt    time.Time  `gorm:"column:first_seen_at;not null"`
	LastSeenAt     time.Time  `gorm:"column:last_seen_at;not null"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
}

// TableName implements schema.Tabler for the jobs table.
func (Job) TableName() string {
	return "jobs"
}

// NewJobModel maps schema.Job to the GORM row shape.
// CreatedAt/UpdatedAt are left zero until persistence sets them.
func NewJobModel(j schema.Job) Job {
	m := Job{
		ID:             j.ID,
		Sources:        textArray(collectSources(j.Source, j.Sources)),
		Title:          j.Title,
		Company:        j.Company,
		CompanyKey:     j.CompanyKey,
		CompanyWebsite: j.CompanyWebsite,
		URL:            j.URL,
		Description:    j.Description,
		Location:       encodeLocation(j.Location),
		Openings:       encodeOpenings(j.Openings),
		SalaryRaw:      j.SalaryRaw,
		Tags:           encodeJobTags(j.Tags),
		Position:       j.Position,
		FirstSeenAt:    j.FirstSeenAt,
		LastSeenAt:     j.LastSeenAt,
	}
	if j.ApplyURL != "" {
		u := j.ApplyURL
		m.ApplyURL = &u
	}
	if !j.PostedAt.IsZero() {
		t := j.PostedAt
		m.PostedAt = &t
	}
	return m
}

func collectSources(source string, sources []string) []string {
	return mergeSourceIDs(nil, source, sources)
}

func mergeSourceIDs(existing []string, source string, more []string) []string {
	var out []string
	seen := make(map[string]struct{})
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range existing {
		add(s)
	}
	add(source)
	for _, s := range more {
		add(s)
	}
	return out
}

type locationJSON struct {
	Type      string   `json:"type"`
	Regions   []string `json:"regions"`
	Countries []string `json:"countries"`
	Timezone  string   `json:"timezone"`
	Raw       string   `json:"raw"`
}

func encodeLocation(loc schema.Location) []byte {
	payload := locationJSON{
		Type:      loc.Type,
		Regions:   nonNilStrings(loc.Regions),
		Countries: nonNilStrings(loc.Countries),
		Timezone:  loc.Timezone,
		Raw:       loc.Raw,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"type":"","regions":[],"countries":[],"timezone":"","raw":""}`)
	}
	return b
}

func decodeLocation(b []byte) schema.Location {
	if len(b) == 0 {
		return schema.Location{}
	}
	var payload locationJSON
	if err := json.Unmarshal(b, &payload); err != nil {
		return schema.Location{}
	}
	return schema.Location{
		Type:      payload.Type,
		Regions:   nilIfEmpty(payload.Regions),
		Countries: nilIfEmpty(payload.Countries),
		Timezone:  payload.Timezone,
		Raw:       payload.Raw,
	}
}

type openingJSON struct {
	Country  string `json:"country"`
	URL      string `json:"url"`
	ApplyURL string `json:"apply_url"`
}

// encodeOpenings stores a nil slice as [], not JSON null.
func encodeOpenings(in []schema.Opening) []byte {
	if len(in) == 0 {
		return []byte("[]")
	}
	payload := make([]openingJSON, len(in))
	for i, opening := range in {
		payload[i] = openingJSON{
			Country:  opening.Country,
			URL:      opening.URL,
			ApplyURL: opening.ApplyURL,
		}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func decodeOpenings(b []byte) []schema.Opening {
	if len(b) == 0 {
		return nil
	}
	var payload []openingJSON
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil
	}
	if len(payload) == 0 {
		return nil
	}
	out := make([]schema.Opening, len(payload))
	for i, opening := range payload {
		out[i] = schema.Opening{
			Country:  opening.Country,
			URL:      opening.URL,
			ApplyURL: opening.ApplyURL,
		}
	}
	return out
}

func encodeJobTags(tags []string) []byte {
	if len(tags) == 0 {
		return []byte("[]")
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func decodeJobTags(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	var tags []string
	if err := json.Unmarshal(b, &tags); err != nil {
		return nil
	}
	return nilIfEmpty(tags)
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func nilIfEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return in
}

// ToDomain maps this row to schema.Job.
func (m *Job) ToDomain() schema.Job {
	j := schema.Job{
		ID:             m.ID,
		Sources:        nilIfEmpty([]string(m.Sources)),
		Title:          m.Title,
		Company:        m.Company,
		CompanyKey:     m.CompanyKey,
		CompanyWebsite: m.CompanyWebsite,
		URL:            m.URL,
		Description:    m.Description,
		Location:       decodeLocation(m.Location),
		Openings:       decodeOpenings(m.Openings),
		SalaryRaw:      m.SalaryRaw,
		Tags:           decodeJobTags(m.Tags),
		Position:       m.Position,
		ClosedAt:       m.ClosedAt,
		FirstSeenAt:    m.FirstSeenAt,
		LastSeenAt:     m.LastSeenAt,
	}
	if m.ApplyURL != nil {
		j.ApplyURL = *m.ApplyURL
	}
	if m.PostedAt != nil {
		j.PostedAt = *m.PostedAt
	}
	return j
}
