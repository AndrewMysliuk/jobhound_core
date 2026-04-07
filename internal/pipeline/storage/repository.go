package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrInvalidRunJobStatus is returned when status is not one of the contract enum strings.
	ErrInvalidRunJobStatus = errors.New("invalid pipeline run job status")
	// ErrInvalidRunJobTransition is returned when SetRunJobStatus would violate allowed transitions.
	ErrInvalidRunJobTransition = errors.New("invalid pipeline run job status transition")
)

// Repository implements pipeline.PipelineRunRepository using GORM.
type Repository struct {
	get pgsql.GormGetter
}

var _ pipeline.PipelineRunRepository = (*Repository)(nil)

// NewRepository wires pipeline run persistence. Pass pgsql.NewGetter(gdb) from pgsql.Open / tests.
func NewRepository(get pgsql.GormGetter) *Repository {
	return &Repository{get: get}
}

// CreateRun inserts a pipeline_runs row and returns its surrogate id.
func (r *Repository) CreateRun(ctx context.Context, slotID *uuid.UUID) (int64, error) {
	run := PipelineRun{CreatedAt: time.Now().UTC()}
	if slotID != nil {
		s := slotID.String()
		run.SlotID = &s
	}
	if err := r.get().WithContext(ctx).Create(&run).Error; err != nil {
		return 0, err
	}
	return run.ID, nil
}

// SetBroadFilterKeyHash implements [pipeline.PipelineRunRepository.SetBroadFilterKeyHash].
func (r *Repository) SetBroadFilterKeyHash(ctx context.Context, pipelineRunID int64, hash string) error {
	if pipelineRunID <= 0 {
		return fmt.Errorf("pipeline run id is required")
	}
	h := strings.TrimSpace(strings.ToLower(hash))
	if h == "" {
		return nil
	}
	return r.get().WithContext(ctx).Model(&PipelineRun{}).
		Where("id = ?", pipelineRunID).
		Update("broad_filter_key_hash", h).Error
}

// SetRunJobStatus inserts a new per-run row (stage-2 outcome) or sets terminal stage-3 on stage2 passed rows.
// Allowed: (no row) → REJECTED_STAGE_2 | PASSED_STAGE_2; stage2 passed + no stage3 → PASSED_STAGE_3 | REJECTED_STAGE_3.
// Repeating the same outcome is a no-op. When a terminal stage-3 outcome exists, stage-2 writes are ignored (Temporal retry idempotency).
func (r *Repository) SetRunJobStatus(ctx context.Context, pipelineRunID int64, jobID string, status pipeline.RunJobStatus) error {
	if jobID == "" {
		return fmt.Errorf("job id is required")
	}
	if !status.Valid() {
		return ErrInvalidRunJobStatus
	}
	isStage2 := status == pipeline.RunJobRejectedStage2 || status == pipeline.RunJobPassedStage2
	isStage3 := status == pipeline.RunJobPassedStage3 || status == pipeline.RunJobRejectedStage3
	if !isStage2 && !isStage3 {
		return ErrInvalidRunJobStatus
	}

	db := r.get().WithContext(ctx)

	var row PipelineRunJob
	err := db.Where("pipeline_run_id = ? AND job_id = ?", pipelineRunID, jobID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !isStage2 {
			return ErrInvalidRunJobTransition
		}
		return db.Create(&PipelineRunJob{
			PipelineRunID: pipelineRunID,
			JobID:         jobID,
			Stage2Status:  string(status),
			Stage3Status:  nil,
		}).Error
	}
	if err != nil {
		return err
	}

	if isStage3 {
		if row.Stage2Status != string(pipeline.RunJobPassedStage2) {
			return ErrInvalidRunJobTransition
		}
		if row.Stage3Status != nil && *row.Stage3Status == string(status) {
			return nil
		}
		if row.Stage3Status != nil && *row.Stage3Status != string(status) {
			return ErrInvalidRunJobTransition
		}
		st := string(status)
		return db.Model(&PipelineRunJob{}).
			Where("pipeline_run_id = ? AND job_id = ?", pipelineRunID, jobID).
			Update("stage3_status", st).Error
	}

	// Stage 2 update
	if row.Stage3Status != nil && *row.Stage3Status != "" {
		return nil
	}
	if row.Stage2Status == string(status) {
		return nil
	}
	return db.Model(&PipelineRunJob{}).
		Where("pipeline_run_id = ? AND job_id = ?", pipelineRunID, jobID).
		Update("stage2_status", string(status)).Error
}

// SetRunJobStage3Rationale implements [pipeline.PipelineRunRepository.SetRunJobStage3Rationale].
func (r *Repository) SetRunJobStage3Rationale(ctx context.Context, pipelineRunID int64, jobID string, rationale string) error {
	if pipelineRunID <= 0 || jobID == "" {
		return fmt.Errorf("pipeline run id and job id are required")
	}
	rat := strings.TrimSpace(rationale)
	var v any
	if rat == "" {
		v = nil
	} else {
		v = rat
	}
	return r.get().WithContext(ctx).Model(&PipelineRunJob{}).
		Where("pipeline_run_id = ? AND job_id = ?", pipelineRunID, jobID).
		Update("stage3_rationale", v).Error
}

// GetRunJobStatus implements [pipeline.PipelineRunRepository.GetRunJobStatus].
func (r *Repository) GetRunJobStatus(ctx context.Context, pipelineRunID int64, jobID string) (pipeline.RunJobStatus, bool, error) {
	if jobID == "" {
		return "", false, fmt.Errorf("job id is required")
	}
	var row PipelineRunJob
	err := r.get().WithContext(ctx).Where("pipeline_run_id = ? AND job_id = ?", pipelineRunID, jobID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	st := pipeutils.EffectiveRunJobStatusFromRow(row.Stage2Status, row.Stage3Status)
	if !st.Valid() {
		return "", false, ErrInvalidRunJobStatus
	}
	return st, true, nil
}

const sqlListPassedStage2JobIDs = `
SELECT prj.job_id FROM pipeline_run_jobs AS prj
INNER JOIN jobs ON jobs.id = prj.job_id
WHERE prj.pipeline_run_id = ? AND prj.stage2_status = ? AND prj.stage3_status IS NULL
ORDER BY CASE WHEN jobs.posted_at IS NULL THEN 1 ELSE 0 END ASC, jobs.posted_at DESC, jobs.id ASC`

// ListPassedStage2JobIDs returns job ids eligible for stage 3: stage2 passed, stage3 not yet terminal (008 ordering).
func (r *Repository) ListPassedStage2JobIDs(ctx context.Context, pipelineRunID int64) ([]string, error) {
	var ids []string
	err := r.get().WithContext(ctx).Raw(sqlListPassedStage2JobIDs,
		pipelineRunID, string(pipeline.RunJobPassedStage2)).Scan(&ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// InvalidateStage3SnapshotsForSlot implements [pipeline.PipelineRunRepository.InvalidateStage3SnapshotsForSlot].
func (r *Repository) InvalidateStage3SnapshotsForSlot(ctx context.Context, slotID uuid.UUID) (int64, error) {
	if slotID == uuid.Nil {
		return 0, fmt.Errorf("slot id is required")
	}
	s := slotID.String()
	db := r.get().WithContext(ctx)
	sub := db.Model(&PipelineRun{}).Select("id").Where("slot_id = ?", s)
	res := db.Model(&PipelineRunJob{}).
		Where("pipeline_run_id IN (?)", sub).
		Where("stage3_status IN ?", []string{
			string(pipeline.RunJobPassedStage3),
			string(pipeline.RunJobRejectedStage3),
		}).
		Updates(map[string]any{
			"stage3_status":    gorm.Expr("NULL"),
			"stage3_rationale": gorm.Expr("NULL"),
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// InvalidateStage2And3SnapshotsForSlot implements [pipeline.PipelineRunRepository.InvalidateStage2And3SnapshotsForSlot].
func (r *Repository) InvalidateStage2And3SnapshotsForSlot(ctx context.Context, slotID uuid.UUID) (int64, error) {
	if slotID == uuid.Nil {
		return 0, fmt.Errorf("slot id is required")
	}
	res := r.get().WithContext(ctx).Where("slot_id = ?", slotID.String()).Delete(&PipelineRun{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// LatestPipelineRunIDForSlot implements [pipeline.PipelineRunRepository.LatestPipelineRunIDForSlot].
func (r *Repository) LatestPipelineRunIDForSlot(ctx context.Context, slotID uuid.UUID) (int64, bool, error) {
	if slotID == uuid.Nil {
		return 0, false, fmt.Errorf("slot id is required")
	}
	var run PipelineRun
	err := r.get().WithContext(ctx).
		Where("slot_id = ?", slotID.String()).
		Order("id DESC").
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return run.ID, true, nil
}

// ManualPatchStage2Bucket implements [pipeline.PipelineRunRepository.ManualPatchStage2Bucket].
func (r *Repository) ManualPatchStage2Bucket(ctx context.Context, pipelineRunID int64, jobID string, passed bool) error {
	if pipelineRunID <= 0 || jobID == "" {
		return fmt.Errorf("pipeline run id and job id are required")
	}
	want := pipeline.RunJobRejectedStage2
	if passed {
		want = pipeline.RunJobPassedStage2
	}
	res := r.get().WithContext(ctx).Model(&PipelineRunJob{}).
		Where("pipeline_run_id = ? AND job_id = ?", pipelineRunID, jobID).
		Where("stage2_status IN ?", []string{
			string(pipeline.RunJobRejectedStage2),
			string(pipeline.RunJobPassedStage2),
		}).
		Updates(map[string]any{
			"stage2_status":    string(want),
			"stage3_status":    gorm.Expr("NULL"),
			"stage3_rationale": gorm.Expr("NULL"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return pipeline.ErrManualPatchNotInScope
	}
	return nil
}

// ManualPatchStage3Bucket implements [pipeline.PipelineRunRepository.ManualPatchStage3Bucket].
func (r *Repository) ManualPatchStage3Bucket(ctx context.Context, pipelineRunID int64, jobID string, passed bool) error {
	if pipelineRunID <= 0 || jobID == "" {
		return fmt.Errorf("pipeline run id and job id are required")
	}
	want := pipeline.RunJobRejectedStage3
	if passed {
		want = pipeline.RunJobPassedStage3
	}
	res := r.get().WithContext(ctx).Model(&PipelineRunJob{}).
		Where("pipeline_run_id = ? AND job_id = ?", pipelineRunID, jobID).
		Where("stage3_status IN ?", []string{
			string(pipeline.RunJobPassedStage3),
			string(pipeline.RunJobRejectedStage3),
		}).
		Updates(map[string]any{
			"stage3_status":    string(want),
			"stage3_rationale": gorm.Expr("NULL"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return pipeline.ErrManualPatchNotInScope
	}
	return nil
}
