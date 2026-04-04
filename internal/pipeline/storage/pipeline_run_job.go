package storage

// PipelineRunJob is the GORM model for pipeline_run_jobs (007 contract §5).
type PipelineRunJob struct {
	PipelineRunID   int64   `gorm:"column:pipeline_run_id;primaryKey"`
	JobID           string  `gorm:"column:job_id;primaryKey;type:text"`
	Status          string  `gorm:"column:status;type:text;not null"`
	Stage3Rationale *string `gorm:"column:stage3_rationale;type:text"`
}

// TableName is the normative SQL table name.
func (PipelineRunJob) TableName() string {
	return "pipeline_run_jobs"
}
