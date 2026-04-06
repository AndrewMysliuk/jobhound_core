package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	jobsschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const sqlListPassedStage2JobsForRun = `
SELECT jobs.* FROM jobs
INNER JOIN pipeline_run_jobs ON pipeline_run_jobs.job_id = jobs.id
WHERE pipeline_run_jobs.pipeline_run_id = ? AND pipeline_run_jobs.status = ?
ORDER BY CASE WHEN jobs.posted_at IS NULL THEN 1 ELSE 0 END ASC, jobs.posted_at DESC, jobs.id ASC`

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
func (r *Repository) ListSlotJobsPassedStage1(ctx context.Context, slotID uuid.UUID) ([]jobdata.Job, error) {
	if slotID == uuid.Nil {
		return nil, fmt.Errorf("slot id is required")
	}
	var models []Job
	err := r.get().WithContext(ctx).
		Table(Job{}.TableName()).
		Joins("INNER JOIN slot_jobs ON slot_jobs.job_id = "+Job{}.TableName()+".id").
		Where("slot_jobs.slot_id = ? AND "+Job{}.TableName()+".stage1_status = ?", slotID, jobsschema.Stage1StatusPassed).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]jobdata.Job, 0, len(models))
	for i := range models {
		out = append(out, models[i].ToDomain())
	}
	return out, nil
}

// ListPassedStage2JobsForRun implements [jobs.JobRepository.ListPassedStage2JobsForRun].
func (r *Repository) ListPassedStage2JobsForRun(ctx context.Context, pipelineRunID int64) ([]jobdata.Job, error) {
	if pipelineRunID <= 0 {
		return nil, fmt.Errorf("pipeline run id is required")
	}
	st := string(pipeline.RunJobPassedStage2)
	var models []Job
	err := r.get().WithContext(ctx).Raw(sqlListPassedStage2JobsForRun, pipelineRunID, st).Scan(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]jobdata.Job, 0, len(models))
	for i := range models {
		out = append(out, models[i].ToDomain())
	}
	return out, nil
}

// jobListScanRow is a jobs row plus slot_jobs.first_seen_at (aliased for Scan).
type jobListScanRow struct {
	Job
	SlotFirstSeenAt    time.Time `gorm:"column:sj_first_seen"`
	PRJStatus          string    `gorm:"column:prj_status"`
	PRJStage3Rationale *string   `gorm:"column:prj_stage3_rationale"`
}

func jobListOrderSQL(jt string) string {
	return "CASE WHEN " + jt + ".posted_at IS NULL THEN 1 ELSE 0 END ASC, " + jt + ".posted_at DESC, " + jt + ".id ASC"
}

func (r *Repository) stage1JobListBase(ctx context.Context, slotID uuid.UUID) *gorm.DB {
	jt := Job{}.TableName()
	return r.get().WithContext(ctx).Table(jt).
		Joins("INNER JOIN slot_jobs ON slot_jobs.job_id = "+jt+".id AND slot_jobs.slot_id = ?", slotID).
		Where(jt+".stage1_status = ?", jobsschema.Stage1StatusPassed)
}

func (r *Repository) stage2JobListBase(ctx context.Context, slotID uuid.UUID, runID int64, statusFilter string) *gorm.DB {
	jt := Job{}.TableName()
	q := r.get().WithContext(ctx).Table(jt).
		Joins("INNER JOIN slot_jobs ON slot_jobs.job_id = "+jt+".id AND slot_jobs.slot_id = ?", slotID).
		Joins("INNER JOIN pipeline_run_jobs prj ON prj.job_id = " + jt + ".id")
	if strings.TrimSpace(statusFilter) == "" {
		return q.Where("prj.pipeline_run_id = ? AND prj.status IN ?", runID, []string{
			string(pipeline.RunJobPassedStage2),
			string(pipeline.RunJobRejectedStage2),
		})
	}
	return q.Where("prj.pipeline_run_id = ? AND prj.status = ?", runID, statusFilter)
}

func (r *Repository) stage3JobListBase(ctx context.Context, slotID uuid.UUID, runID int64, statusFilter string) *gorm.DB {
	jt := Job{}.TableName()
	q := r.get().WithContext(ctx).Table(jt).
		Joins("INNER JOIN slot_jobs ON slot_jobs.job_id = "+jt+".id AND slot_jobs.slot_id = ?", slotID).
		Joins("INNER JOIN pipeline_run_jobs prj ON prj.job_id = " + jt + ".id")
	if strings.TrimSpace(statusFilter) == "" {
		return q.Where("prj.pipeline_run_id = ? AND prj.status IN ?", runID, []string{
			string(pipeline.RunJobPassedStage3),
			string(pipeline.RunJobRejectedStage3),
		})
	}
	return q.Where("prj.pipeline_run_id = ? AND prj.status = ?", runID, statusFilter)
}

func (r *Repository) countAndListJobEntries(base *gorm.DB, jt string, offset, limit int, prjExtraSelect string) ([]jobsschema.JobListEntry, int64, error) {
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	sel := jt + ".*, slot_jobs.first_seen_at AS sj_first_seen"
	if prjExtraSelect != "" {
		sel += ", " + prjExtraSelect
	}
	var rows []jobListScanRow
	err := base.Session(&gorm.Session{}).
		Select(sel).
		Order(jobListOrderSQL(jt)).
		Offset(offset).
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]jobsschema.JobListEntry, 0, len(rows))
	for i := range rows {
		ent := jobsschema.JobListEntry{
			Job:               rows[i].ToDomain(),
			FirstSeenAt:       rows[i].SlotFirstSeenAt.UTC(),
			PipelineRunStatus: rows[i].PRJStatus,
		}
		if rows[i].PRJStage3Rationale != nil && *rows[i].PRJStage3Rationale != "" {
			ent.Stage3Rationale = rows[i].PRJStage3Rationale
		}
		out = append(out, ent)
	}
	return out, total, nil
}

// ListSlotStage1Jobs implements [jobs.JobRepository.ListSlotStage1Jobs].
func (r *Repository) ListSlotStage1Jobs(ctx context.Context, slotID uuid.UUID, offset, limit int) ([]jobsschema.JobListEntry, int64, error) {
	if slotID == uuid.Nil {
		return nil, 0, fmt.Errorf("slot id is required")
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("limit must be positive")
	}
	if offset < 0 {
		return nil, 0, fmt.Errorf("offset must be non-negative")
	}
	jt := Job{}.TableName()
	return r.countAndListJobEntries(r.stage1JobListBase(ctx, slotID), jt, offset, limit, "")
}

// ListPipelineRunStage2Jobs implements [jobs.JobRepository.ListPipelineRunStage2Jobs].
func (r *Repository) ListPipelineRunStage2Jobs(ctx context.Context, slotID uuid.UUID, pipelineRunID int64, statusFilter string, offset, limit int) ([]jobsschema.JobListEntry, int64, error) {
	if slotID == uuid.Nil {
		return nil, 0, fmt.Errorf("slot id is required")
	}
	if pipelineRunID <= 0 {
		return nil, 0, fmt.Errorf("pipeline run id is required")
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("limit must be positive")
	}
	if offset < 0 {
		return nil, 0, fmt.Errorf("offset must be non-negative")
	}
	jt := Job{}.TableName()
	return r.countAndListJobEntries(r.stage2JobListBase(ctx, slotID, pipelineRunID, statusFilter), jt, offset, limit, "prj.status AS prj_status")
}

// ListPipelineRunStage3Jobs implements [jobs.JobRepository.ListPipelineRunStage3Jobs].
func (r *Repository) ListPipelineRunStage3Jobs(ctx context.Context, slotID uuid.UUID, pipelineRunID int64, statusFilter string, offset, limit int) ([]jobsschema.JobListEntry, int64, error) {
	if slotID == uuid.Nil {
		return nil, 0, fmt.Errorf("slot id is required")
	}
	if pipelineRunID <= 0 {
		return nil, 0, fmt.Errorf("pipeline run id is required")
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("limit must be positive")
	}
	if offset < 0 {
		return nil, 0, fmt.Errorf("offset must be non-negative")
	}
	jt := Job{}.TableName()
	return r.countAndListJobEntries(r.stage3JobListBase(ctx, slotID, pipelineRunID, statusFilter), jt, offset, limit,
		"prj.status AS prj_status, prj.stage3_rationale AS prj_stage3_rationale")
}
