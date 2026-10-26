package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	jobschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	scoringschema "github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
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
// Aggregator upserts refresh last_seen_at / openings / empty company_website
// but never clear closed_at when closed_by is ATS — only ReopenPresent does.
func (r *Repository) SaveIngest(ctx context.Context, job jobdata.Job) (skipped bool, err error) {
	if job.ID == "" {
		return false, fmt.Errorf("job id is required")
	}
	now := time.Now().UTC()
	var existing Job
	err = r.get().WithContext(ctx).Where("id = ?", job.ID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row := newIngestJob(job, now)
		m := NewJobModel(row)
		m.CreatedAt = now
		m.UpdatedAt = now
		return false, r.get().WithContext(ctx).Create(&m).Error
	}

	merged := mergeIngestJob(existing.ToDomain(), job, now)
	m := NewJobModel(merged)
	m.CreatedAt = existing.CreatedAt
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	// Close protection: preserve close columns, company_key_ref, and board_seen_at on ingest.
	m.ClosedAt = existing.ClosedAt
	m.ClosedBy = existing.ClosedBy
	m.CompanyKeyRef = existing.CompanyKeyRef
	m.BoardSeenAt = existing.BoardSeenAt
	return false, r.get().WithContext(ctx).Save(&m).Error
}

// CloseAbsent sets closed_at / closed_by=ATS on open jobs for companyKey whose
// id is not in seenIDs and that a board poll has seen at least once
// (board_seen_at IS NOT NULL). Aggregator-only rows linked via company_key_ref
// stay open for the last_seen_at window (R1.4 a). An empty seenIDs closes every
// board-seen open job for the company.
func (r *Repository) CloseAbsent(ctx context.Context, companyKey string, seenIDs []string) (int64, error) {
	if companyKey == "" {
		return 0, fmt.Errorf("company key is required")
	}
	now := time.Now().UTC()
	q := r.get().WithContext(ctx).Model(&Job{}).
		Where("company_key_ref = ? AND closed_at IS NULL AND board_seen_at IS NOT NULL", companyKey)
	if len(seenIDs) > 0 {
		q = q.Where("id NOT IN ?", seenIDs)
	}
	res := q.Updates(map[string]any{
		"closed_at":  now,
		"closed_by":  string(jobschema.ClosedByATS),
		"updated_at": now,
	})
	return res.RowsAffected, res.Error
}

// ReopenPresent clears closed_at / closed_by for jobs of companyKey whose id is
// in seenIDs. Used after a complete poll that lists the vacancy again.
func (r *Repository) ReopenPresent(ctx context.Context, companyKey string, seenIDs []string) (int64, error) {
	if companyKey == "" {
		return 0, fmt.Errorf("company key is required")
	}
	if len(seenIDs) == 0 {
		return 0, nil
	}
	now := time.Now().UTC()
	res := r.get().WithContext(ctx).
		Exec(
			`UPDATE jobs SET closed_at = NULL, closed_by = '', updated_at = ? WHERE company_key_ref = ? AND id IN ? AND closed_at IS NOT NULL`,
			now, companyKey, seenIDs,
		)
	return res.RowsAffected, res.Error
}

// UpsertBoardJobs writes vacancies from a company board poll and sets
// company_key_ref and board_seen_at. Closed columns are left alone; callers
// use ReopenPresent / CloseAbsent. Non-empty board description/title replace
// aggregator text so scoring sees ATS content.
//
// Known posted_at strictly older than PostedAtRejectDays is skipped (not
// written, not updated). Zero posted_at is written as usual. A skipped id is
// absent from the caller's seen set, so on a COMPLETE poll CloseAbsent closes
// an existing board_seen row — intentional: stale vacancies are not kept open
// by refreshing last_seen_at / board_seen_at every poll.
func (r *Repository) UpsertBoardJobs(ctx context.Context, companyKey string, boardJobs []jobdata.Job) error {
	if companyKey == "" {
		return fmt.Errorf("company key is required")
	}
	now := time.Now().UTC()
	keyRef := companyKey
	seenAt := now
	cutoff := now.Add(-time.Duration(scoringschema.PostedAtRejectDays) * 24 * time.Hour)
	for _, job := range boardJobs {
		if job.ID == "" {
			return fmt.Errorf("job id is required")
		}
		if !job.PostedAt.IsZero() && job.PostedAt.UTC().Before(cutoff) {
			continue
		}
		var existing Job
		err := r.get().WithContext(ctx).Where("id = ?", job.ID).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row := newIngestJob(job, now)
			m := NewJobModel(row)
			m.CompanyKeyRef = &keyRef
			m.BoardSeenAt = &seenAt
			m.CreatedAt = now
			m.UpdatedAt = now
			if err := r.get().WithContext(ctx).Create(&m).Error; err != nil {
				return err
			}
			continue
		}
		merged := mergeBoardJob(existing.ToDomain(), job, now)
		m := NewJobModel(merged)
		m.CompanyKeyRef = &keyRef
		m.BoardSeenAt = &seenAt
		m.CreatedAt = existing.CreatedAt
		if m.CreatedAt.IsZero() {
			m.CreatedAt = now
		}
		m.UpdatedAt = now
		m.ClosedAt = existing.ClosedAt
		m.ClosedBy = existing.ClosedBy
		if err := r.get().WithContext(ctx).Save(&m).Error; err != nil {
			return err
		}
	}
	return nil
}

// BackfillCompanyKeyRef sets company_key_ref on every job whose content
// company_key is in contentKeys. Idempotent; closed_at / closed_by are not
// touched. Used by company discover after merge-or-insert.
func (r *Repository) BackfillCompanyKeyRef(ctx context.Context, companyKey string, contentKeys []string) (int64, error) {
	if companyKey == "" {
		return 0, fmt.Errorf("company key is required")
	}
	keys := uniqueNonEmptyStrings(contentKeys)
	if len(keys) == 0 {
		return 0, nil
	}
	now := time.Now().UTC()
	res := r.get().WithContext(ctx).Model(&Job{}).
		Where("company_key IN ? AND (company_key_ref IS NULL OR company_key_ref != ?)", keys, companyKey).
		Updates(map[string]any{
			"company_key_ref": companyKey,
			"updated_at":      now,
		})
	return res.RowsAffected, res.Error
}

func uniqueNonEmptyStrings(vals []string) []string {
	seen := make(map[string]struct{}, len(vals))
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// CountOpenByCompanyKeyRef counts jobs linked to companyKey that are still open.
func (r *Repository) CountOpenByCompanyKeyRef(ctx context.Context, companyKey string) (int64, error) {
	if companyKey == "" {
		return 0, fmt.Errorf("company key is required")
	}
	var n int64
	err := r.get().WithContext(ctx).Model(&Job{}).
		Where("company_key_ref = ? AND closed_at IS NULL", companyKey).
		Count(&n).Error
	return n, err
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

// ListEnrichCandidates implements [jobs.JobRepository.ListEnrichCandidates].
func (r *Repository) ListEnrichCandidates(ctx context.Context, since, postedAfter time.Time) ([]jobdata.Job, error) {
	var rows []Job
	err := r.get().WithContext(ctx).
		Where("first_seen_at >= ? AND closed_at IS NULL AND (posted_at IS NULL OR posted_at >= ?)",
			since.UTC(), postedAfter.UTC()).
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

// ListLastSeenSince implements [jobs.JobRepository.ListLastSeenSince].
func (r *Repository) ListLastSeenSince(ctx context.Context, since time.Time) ([]jobdata.Job, error) {
	var rows []Job
	err := r.get().WithContext(ctx).
		Where("last_seen_at >= ?", since.UTC()).
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

// ListPostedBefore implements [jobs.JobRepository.ListPostedBefore].
func (r *Repository) ListPostedBefore(ctx context.Context, cutoff time.Time) ([]jobdata.Job, error) {
	var rows []Job
	err := r.get().WithContext(ctx).
		Where("posted_at IS NOT NULL AND posted_at < ?", cutoff.UTC()).
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
