package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
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
func (r *Repository) CreateRun(ctx context.Context) (int64, error) {
	run := PipelineRun{CreatedAt: time.Now().UTC()}
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

// ListPassedStage2JobIDs returns job ids in PASSED_STAGE_2 for this run.
// Ordering is by job_id ascending (deterministic; cap selection may use another order in a later layer).
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
