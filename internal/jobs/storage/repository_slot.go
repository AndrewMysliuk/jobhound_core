package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

const sqlListPassedStage2JobsForRun = `
SELECT jobs.* FROM jobs
INNER JOIN pipeline_run_jobs ON pipeline_run_jobs.job_id = jobs.id
WHERE pipeline_run_jobs.pipeline_run_id = ? AND pipeline_run_jobs.status = ?
ORDER BY CASE WHEN jobs.posted_at IS NULL THEN 1 ELSE 0 END ASC, jobs.posted_at DESC`

// UpsertSlotJob implements [jobs.JobRepository.UpsertSlotJob].
func (r *Repository) UpsertSlotJob(ctx context.Context, slotID uuid.UUID, jobID string) error {
	if jobID == "" {
		return fmt.Errorf("job id is required")
	}
	if slotID == uuid.Nil {
		return fmt.Errorf("slot id is required")
	}
	row := SlotJob{
		SlotID:      slotID,
		JobID:       jobID,
		FirstSeenAt: time.Now().UTC(),
	}
	return r.get().WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "slot_id"}, {Name: "job_id"}},
		DoNothing: true,
	}).Create(&row).Error
}

// ListSlotJobsPassedStage1 implements [jobs.JobRepository.ListSlotJobsPassedStage1].
func (r *Repository) ListSlotJobsPassedStage1(ctx context.Context, slotID uuid.UUID) ([]domain.Job, error) {
	if slotID == uuid.Nil {
		return nil, fmt.Errorf("slot id is required")
	}
	var models []Job
	err := r.get().WithContext(ctx).
		Table(Job{}.TableName()).
		Joins("INNER JOIN slot_jobs ON slot_jobs.job_id = "+Job{}.TableName()+".id").
		Where("slot_jobs.slot_id = ? AND "+Job{}.TableName()+".stage1_status = ?", slotID, jobs.Stage1StatusPassed).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Job, 0, len(models))
	for i := range models {
		out = append(out, models[i].ToDomain())
	}
	return out, nil
}

// ListPassedStage2JobsForRun implements [jobs.JobRepository.ListPassedStage2JobsForRun].
func (r *Repository) ListPassedStage2JobsForRun(ctx context.Context, pipelineRunID int64) ([]domain.Job, error) {
	if pipelineRunID <= 0 {
		return nil, fmt.Errorf("pipeline run id is required")
	}
	st := string(pipeline.RunJobPassedStage2)
	var models []Job
	err := r.get().WithContext(ctx).Raw(sqlListPassedStage2JobsForRun, pipelineRunID, st).Scan(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Job, 0, len(models))
	for i := range models {
		out = append(out, models[i].ToDomain())
	}
	return out, nil
}
