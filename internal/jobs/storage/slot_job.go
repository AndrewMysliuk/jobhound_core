package storage

import (
	"time"

	"github.com/google/uuid"
)

// SlotJob is the GORM model for slot_jobs (specs/008-manual-search-workflow).
type SlotJob struct {
	SlotID      uuid.UUID `gorm:"column:slot_id;type:uuid;not null;primaryKey"`
	JobID       string    `gorm:"column:job_id;type:text;not null;primaryKey"`
	FirstSeenAt time.Time `gorm:"column:first_seen_at;not null"`
}

// TableName implements schema.Tabler for the slot_jobs table.
func (SlotJob) TableName() string {
	return "slot_jobs"
}
