package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
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
func (r *Repository) Save(ctx context.Context, job jobdata.Job) error {
	if job.ID == "" {
		return fmt.Errorf("job id is required")
	}
	m := NewJobModel(job)
	return r.get().WithContext(ctx).Save(&m).Error
}

// SaveIngest implements [jobs.JobRepository.SaveIngest].
func (r *Repository) SaveIngest(ctx context.Context, job jobdata.Job) (skipped bool, err error) {
	if job.ID == "" {
		return false, fmt.Errorf("job id is required")
	}
	now := time.Now().UTC()
	existing, err := r.GetByID(ctx, job.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return false, err
	}
	if errors.Is(err, ErrNotFound) {
		row := job
		row.Sources = mergeSourceIDs(nil, job.Source, job.Sources)
		row.FirstSeenAt = now
		row.LastSeenAt = now
		m := NewJobModel(row)
		m.CreatedAt = now
		m.UpdatedAt = now
		return false, r.get().WithContext(ctx).Create(&m).Error
	}

	merged := mergeIngestJob(existing, job, now)
	m := NewJobModel(merged)
	m.CreatedAt = existingRowCreatedAt(ctx, r, job.ID)
	m.UpdatedAt = now
	return false, r.get().WithContext(ctx).Save(&m).Error
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
func (r *Repository) GetByID(ctx context.Context, id string) (jobdata.Job, error) {
	if id == "" {
		return jobdata.Job{}, fmt.Errorf("job id is required")
	}
	var m Job
	err := r.get().WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return jobdata.Job{}, ErrNotFound
		}
		return jobdata.Job{}, err
	}
	return m.ToDomain(), nil
}

// ListFirstSeenSince implements [jobs.JobRepository.ListFirstSeenSince].
func (r *Repository) ListFirstSeenSince(ctx context.Context, since time.Time) ([]jobdata.Job, error) {
	var rows []Job
	err := r.get().WithContext(ctx).
		Where("first_seen_at >= ?", since.UTC()).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]jobdata.Job, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].ToDomain())
	}
	return out, nil
}

// DeleteJobsLastSeenBeforeUTC implements [jobs.JobRepository.DeleteJobsLastSeenBeforeUTC].
func (r *Repository) DeleteJobsLastSeenBeforeUTC(ctx context.Context, cutoff time.Time) (int64, error) {
	tx := r.get().WithContext(ctx).Where("last_seen_at < ?", cutoff.UTC()).Delete(&Job{})
	return tx.RowsAffected, tx.Error
}
