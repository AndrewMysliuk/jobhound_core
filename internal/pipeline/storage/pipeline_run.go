package storage

import "time"

// PipelineRun is the GORM model for pipeline_runs (007 contract §4).
type PipelineRun struct {
	ID                 int64     `gorm:"column:id;primaryKey;autoIncrement"`
	CreatedAt          time.Time `gorm:"column:created_at;not null"`
	SlotID             *string   `gorm:"column:slot_id"` // UUID canonical string when set (FK to search_slots when that table exists).
	BroadFilterKeyHash *string   `gorm:"column:broad_filter_key_hash"`
}

// TableName is the normative SQL table name.
func (PipelineRun) TableName() string {
	return "pipeline_runs"
}
