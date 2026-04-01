package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/gorm"
)

// ErrNotFound is returned by Repository.GetByID when the id is absent.
var ErrNotFound = errors.New("job not found")

// Repository persists jobs via GORM (jobs.JobRepository). Ingest/list/query extras belong in 006+.
type Repository struct {
	get pgsql.GormGetter
}

var _ jobs.JobRepository = (*Repository)(nil)

// NewRepository wires job persistence. Pass pgsql.NewGetter(gdb) from pgsql.Open / OpenFromEnv.
func NewRepository(get pgsql.GormGetter) *Repository {
	return &Repository{get: get}
}

// Save inserts or updates a row by primary key id (GORM Save).
func (r *Repository) Save(ctx context.Context, job domain.Job) error {
	if job.ID == "" {
		return fmt.Errorf("job id is required")
	}
	m := NewJobModel(job)
	return r.get().WithContext(ctx).Save(&m).Error
}

// SaveIngest implements [jobs.JobRepository.SaveIngest].
func (r *Repository) SaveIngest(ctx context.Context, job domain.Job) (skipped bool, err error) {
	if job.ID == "" {
		return false, fmt.Errorf("job id is required")
	}
	now := time.Now().UTC()
	want := job
	passed := jobs.Stage1StatusPassed
	want.Stage1Status = &passed

	existing, err := r.GetByID(ctx, want.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return false, err
	}
	if errors.Is(err, ErrNotFound) {
		m := NewJobModel(want)
		m.CreatedAt = now
		m.UpdatedAt = now
		return false, r.get().WithContext(ctx).Create(&m).Error
	}

	existNorm := existing
	existNorm.Stage1Status = normalizeStage1(existing.Stage1Status)
	wantNorm := want
	wantNorm.Stage1Status = normalizeStage1(want.Stage1Status)

	if !jobEqualForIngestSkip(existNorm, wantNorm) {
		m := NewJobModel(want)
		m.CreatedAt = existingRowCreatedAt(ctx, r, want.ID)
		m.UpdatedAt = now
		return false, r.get().WithContext(ctx).Save(&m).Error
	}
	if existing.Description == want.Description {
		return true, nil
	}
	return false, r.get().WithContext(ctx).Model(&Job{}).
		Where("id = ?", want.ID).
		Updates(map[string]interface{}{
			"description": want.Description,
			"updated_at":  now,
		}).Error
}

func existingRowCreatedAt(ctx context.Context, r *Repository, id string) time.Time {
	var m Job
	err := r.get().WithContext(ctx).Select("created_at").Where("id = ?", id).First(&m).Error
	if err != nil || m.CreatedAt.IsZero() {
		return time.Now().UTC()
	}
	return m.CreatedAt
}

// GetByID loads one job by stable id.
func (r *Repository) GetByID(ctx context.Context, id string) (domain.Job, error) {
	if id == "" {
		return domain.Job{}, fmt.Errorf("job id is required")
	}
	var m Job
	err := r.get().WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Job{}, ErrNotFound
		}
		return domain.Job{}, err
	}
	return m.ToDomain(), nil
}

// DeleteJobsCreatedBeforeUTC implements [jobs.JobRepository.DeleteJobsCreatedBeforeUTC].
func (r *Repository) DeleteJobsCreatedBeforeUTC(ctx context.Context, cutoff time.Time) (int64, error) {
	tx := r.get().WithContext(ctx).Where("created_at < ?", cutoff.UTC()).Delete(&Job{})
	return tx.RowsAffected, tx.Error
}
