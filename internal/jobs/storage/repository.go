package storage

import (
	"context"
	"errors"
	"fmt"

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
