package storage

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

// ProfileMatch is the GORM model for profile_matches.
// The list index is (profile_id, bucket, user_status, score DESC).
type ProfileMatch struct {
	ProfileID  string    `gorm:"column:profile_id;primaryKey;type:text;index:profile_matches_list,priority:1"`
	JobID      string    `gorm:"column:job_id;primaryKey;type:text"`
	Bucket     string    `gorm:"column:bucket;type:text;not null;index:profile_matches_list,priority:2"`
	Score      int       `gorm:"column:score;not null;index:profile_matches_list,priority:4,sort:desc"`
	Signals    []byte    `gorm:"column:signals;type:jsonb;not null"`
	UserStatus string    `gorm:"column:user_status;type:text;not null;default:'NEW';index:profile_matches_list,priority:3"`
	RunID      int64     `gorm:"column:run_id;not null"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null"`
}

// TableName implements schema.Tabler.
func (ProfileMatch) TableName() string { return "profile_matches" }

func newProfileMatch(m schema.Match, signals []byte, updatedAt time.Time) ProfileMatch {
	status := m.UserStatus
	if status == "" {
		status = schema.UserStatusNew
	}
	return ProfileMatch{
		ProfileID:  m.ProfileID,
		JobID:      m.JobID,
		Bucket:     string(m.Bucket),
		Score:      m.Score,
		Signals:    signals,
		UserStatus: string(status),
		RunID:      m.RunID,
		UpdatedAt:  updatedAt.UTC(),
	}
}
