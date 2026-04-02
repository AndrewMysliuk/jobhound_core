package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
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

// SetRunJobStatus inserts a new per-run row (stage-2 outcome) or moves PASSED_STAGE_2 to a stage-3 terminal status.
// Allowed: (no row) → REJECTED_STAGE_2 | PASSED_STAGE_2; PASSED_STAGE_2 → PASSED_STAGE_3 | REJECTED_STAGE_3.
// Repeating the same status is a no-op.
func (r *Repository) SetRunJobStatus(ctx context.Context, pipelineRunID int64, jobID string, status pipeline.RunJobStatus) error {
	if jobID == "" {
		return fmt.Errorf("job id is required")
	}
	if !status.Valid() {
		return ErrInvalidRunJobStatus
	}

	db := r.get().WithContext(ctx)

	var row PipelineRunJob
	err := db.Where("pipeline_run_id = ? AND job_id = ?", pipelineRunID, jobID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if status != pipeline.RunJobRejectedStage2 && status != pipeline.RunJobPassedStage2 {
			return ErrInvalidRunJobTransition
		}
		return db.Create(&PipelineRunJob{
			PipelineRunID: pipelineRunID,
			JobID:         jobID,
			Status:        string(status),
		}).Error
	}
	if err != nil {
		return err
	}

	current := pipeline.RunJobStatus(row.Status)
	if current == status {
		return nil
	}
	// Temporal retry: stage-1/2 persistence runs again; do not downgrade terminal stage-3 rows.
	if status == pipeline.RunJobRejectedStage2 || status == pipeline.RunJobPassedStage2 {
		if current == pipeline.RunJobPassedStage3 || current == pipeline.RunJobRejectedStage3 {
			return nil
		}
	}

	switch current {
	case pipeline.RunJobRejectedStage2, pipeline.RunJobPassedStage3, pipeline.RunJobRejectedStage3:
		return ErrInvalidRunJobTransition
	case pipeline.RunJobPassedStage2:
		if status != pipeline.RunJobPassedStage3 && status != pipeline.RunJobRejectedStage3 {
			return ErrInvalidRunJobTransition
		}
		return db.Model(&PipelineRunJob{}).
			Where("pipeline_run_id = ? AND job_id = ?", pipelineRunID, jobID).
			Update("status", string(status)).Error
	default:
		return ErrInvalidRunJobTransition
	}
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
	st := pipeline.RunJobStatus(row.Status)
	if !st.Valid() {
		return "", false, ErrInvalidRunJobStatus
	}
	return st, true, nil
}

// ListPassedStage2JobIDs returns job ids in PASSED_STAGE_2 for this run.
// Ordering is job_id ascending (007 §2 normative cap ordering).
func (r *Repository) ListPassedStage2JobIDs(ctx context.Context, pipelineRunID int64) ([]string, error) {
	var ids []string
	err := r.get().WithContext(ctx).Model(&PipelineRunJob{}).
		Where("pipeline_run_id = ? AND status = ?", pipelineRunID, string(pipeline.RunJobPassedStage2)).
		Order("job_id ASC").
		Pluck("job_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}
