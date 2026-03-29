package storage

import (
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
	}
	if j.ApplyURL != "" {
		u := j.ApplyURL
		m.ApplyURL = &u
	}
	if !j.PostedAt.IsZero() {
		t := j.PostedAt
		m.PostedAt = &t
	}
	if j.UserID != nil && *j.UserID != "" {
		u := *j.UserID
		m.UserID = &u
	}
	return m
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
	}
	if m.ApplyURL != nil {
		j.ApplyURL = *m.ApplyURL
	}
	if m.PostedAt != nil {
		j.PostedAt = *m.PostedAt
	}
	if m.UserID != nil && *m.UserID != "" {
		u := *m.UserID
		j.UserID = &u
	}
	return j
}
