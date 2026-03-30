package storage

import (
	"encoding/json"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

// Job is the GORM model for the jobs table.
// See specs/002-postgres-gorm-migrations/contracts/jobs-schema.md.
type Job struct {
	ID          string     `gorm:"column:id;primaryKey;type:text"`
	Source      string     `gorm:"column:source;type:text;not null"`
	Title       string     `gorm:"column:title;type:text;not null;default:''"`
	Company     string     `gorm:"column:company;type:text;not null;default:''"`
	URL         string     `gorm:"column:url;type:text;not null;default:''"`
	ApplyURL    *string    `gorm:"column:apply_url;type:text"`
	Description string     `gorm:"column:description;type:text;not null;default:''"`
	PostedAt    *time.Time `gorm:"column:posted_at"`
	IsRemote    *bool      `gorm:"column:is_remote"`
	CountryCode string     `gorm:"column:country_code;type:text;not null;default:''"`
	SalaryRaw   string     `gorm:"column:salary_raw;type:text;not null;default:''"`
	Tags        []byte     `gorm:"column:tags;type:jsonb;not null"`
	Position    *string    `gorm:"column:position;type:text"`
	UserID      *string    `gorm:"column:user_id;type:text"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null"`
}

// TableName implements schema.Tabler for the jobs table.
func (Job) TableName() string {
	return "jobs"
}

// NewJobModel maps domain.Job to the GORM row shape (contracts/jobs-schema.md).
// CreatedAt/UpdatedAt are left zero until persistence sets them.
func NewJobModel(j domain.Job) Job {
	m := Job{
		ID:          j.ID,
		Source:      j.Source,
		Title:       j.Title,
		Company:     j.Company,
		URL:         j.URL,
		Description: j.Description,
		SalaryRaw:   j.SalaryRaw,
		Position:    j.Position,
	}
	m.Tags = encodeJobTags(j.Tags)
	if j.ApplyURL != "" {
		u := j.ApplyURL
		m.ApplyURL = &u
	}
	if !j.PostedAt.IsZero() {
		t := j.PostedAt
		m.PostedAt = &t
	}
	if j.Remote != nil {
		r := *j.Remote
		m.IsRemote = &r
	}
	m.CountryCode = j.CountryCode
	if j.UserID != nil && *j.UserID != "" {
		u := *j.UserID
		m.UserID = &u
	}
	return m
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
	if len(tags) == 0 {
		return nil
	}
	return tags
}

// ToDomain maps this row to domain.Job (contracts/jobs-schema.md).
func (m *Job) ToDomain() domain.Job {
	j := domain.Job{
		ID:          m.ID,
		Source:      m.Source,
		Title:       m.Title,
		Company:     m.Company,
		URL:         m.URL,
		Description: m.Description,
		SalaryRaw:   m.SalaryRaw,
		Tags:        decodeJobTags(m.Tags),
		Position:    m.Position,
	}
	if m.ApplyURL != nil {
		j.ApplyURL = *m.ApplyURL
	}
	if m.PostedAt != nil {
		j.PostedAt = *m.PostedAt
	}
	if m.IsRemote != nil {
		r := *m.IsRemote
		j.Remote = &r
	}
	j.CountryCode = m.CountryCode
	if m.UserID != nil && *m.UserID != "" {
		u := *m.UserID
		j.UserID = &u
	}
	return j
}
