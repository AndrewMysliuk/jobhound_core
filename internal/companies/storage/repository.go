package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultListLimit = 50
	maxListLimit     = 100
)

// Repository persists companies rows.
type Repository struct {
	get pgsql.GormGetter
}

// NewRepository wires company persistence.
func NewRepository(get pgsql.GormGetter) *Repository {
	return &Repository{get: get}
}

// Upsert inserts a company or updates mutable fields on key conflict.
// first_seen_at is set on insert only.
func (r *Repository) Upsert(ctx context.Context, c schema.Company) error {
	if c.Key == "" {
		return fmt.Errorf("company key is required")
	}
	if c.Name == "" {
		return fmt.Errorf("company name is required")
	}
	if c.ResolutionStatus == "" {
		return fmt.Errorf("resolution status is required")
	}
	now := time.Now().UTC()
	row := newCompany(c)
	if row.FirstSeenAt.IsZero() {
		row.FirstSeenAt = now
	}
	if row.LastSeenAt.IsZero() {
		row.LastSeenAt = now
	}
	row.CreatedAt = now
	row.UpdatedAt = now
	return r.get().WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "aliases", "website", "ats", "ats_slug", "board_url", "board_etag",
				"resolution_status", "resolution_method", "poll_state", "next_resolve_at",
				"last_seen_at", "last_polled_at", "poll_fail_count", "empty_poll_streak",
				"needs_review", "updated_at",
			}),
		}).
		Create(&row).Error
}

// GetByKey loads one company by its immutable key.
func (r *Repository) GetByKey(ctx context.Context, key string) (schema.Company, error) {
	if key == "" {
		return schema.Company{}, fmt.Errorf("company key is required")
	}
	var row Company
	err := r.get().WithContext(ctx).Where("key = ?", key).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return schema.Company{}, companies.ErrCompanyNotFound
		}
		return schema.Company{}, err
	}
	return row.toDomain(), nil
}

// FindByWebsite returns the company whose website matches exactly.
func (r *Repository) FindByWebsite(ctx context.Context, website string) (schema.Company, error) {
	website = strings.TrimSpace(website)
	if website == "" {
		return schema.Company{}, fmt.Errorf("website is required")
	}
	var row Company
	err := r.get().WithContext(ctx).Where("website = ?", website).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return schema.Company{}, companies.ErrCompanyNotFound
		}
		return schema.Company{}, err
	}
	return row.toDomain(), nil
}

// FindByAlias returns the company that lists alias in aliases.
func (r *Repository) FindByAlias(ctx context.Context, alias string) (schema.Company, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return schema.Company{}, fmt.Errorf("alias is required")
	}
	db := r.get().WithContext(ctx)
	var row Company
	var err error
	if db.Name() == "postgres" {
		err = db.Where("? = ANY(aliases)", alias).First(&row).Error
	} else {
		// SQLite tests store the Postgres text-array literal.
		err = db.Where("aliases LIKE ?", `%"`+escapeLike(alias)+`"%`).First(&row).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return schema.Company{}, companies.ErrCompanyNotFound
		}
		return schema.Company{}, err
	}
	return row.toDomain(), nil
}

func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`, `"`, `\"`)
	return replacer.Replace(s)
}

func (r *Repository) listFilterQuery(ctx context.Context, f schema.ListFilter) *gorm.DB {
	q := r.get().WithContext(ctx).Model(&Company{})
	if f.ResolutionStatus != "" {
		q = q.Where("resolution_status = ?", string(f.ResolutionStatus))
	}
	if f.PollState != "" {
		q = q.Where("poll_state = ?", string(f.PollState))
	}
	if f.ATS != "" {
		q = q.Where("ats = ?", string(f.ATS))
	}
	if f.NeedsReview != nil {
		q = q.Where("needs_review = ?", *f.NeedsReview)
	}
	return q
}

// List returns one page of companies matching the filter.
func (r *Repository) List(ctx context.Context, f schema.ListFilter) ([]schema.Company, error) {
	page := f.Page
	if page < 1 {
		page = 1
	}
	limit := f.Limit
	if limit < 1 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	var rows []Company
	err := r.listFilterQuery(ctx, f).
		Order("CASE WHEN last_polled_at IS NULL THEN 0 ELSE 1 END, last_polled_at ASC, key ASC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]schema.Company, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toDomain())
	}
	return out, nil
}

// Count returns how many companies match the filter (ignoring page/limit).
func (r *Repository) Count(ctx context.Context, f schema.ListFilter) (int, error) {
	var total int64
	if err := r.listFilterQuery(ctx, f).Count(&total).Error; err != nil {
		return 0, err
	}
	return int(total), nil
}

// ListPendingResolve returns PENDING companies due for resolve (next_resolve_at
// null or at/before now), oldest first, capped.
func (r *Repository) ListPendingResolve(ctx context.Context, now time.Time, limit int) ([]schema.Company, error) {
	if limit < 1 {
		return nil, nil
	}
	var rows []Company
	err := r.get().WithContext(ctx).
		Where("resolution_status = ?", string(schema.ResolutionStatusPending)).
		Where("next_resolve_at IS NULL OR next_resolve_at <= ?", now.UTC()).
		Order("first_seen_at ASC, key ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]schema.Company, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toDomain())
	}
	return out, nil
}

// ListDuePollable returns RESOLVED + ACTIVE companies whose last_polled_at is
// null or strictly before dueBefore, oldest first (NULLS FIRST), capped.
func (r *Repository) ListDuePollable(ctx context.Context, dueBefore time.Time, limit int) ([]schema.Company, error) {
	if limit < 1 {
		return nil, nil
	}
	var rows []Company
	err := r.get().WithContext(ctx).
		Where("resolution_status = ?", string(schema.ResolutionStatusResolved)).
		Where("poll_state = ?", string(schema.PollStateActive)).
		Where("last_polled_at IS NULL OR last_polled_at < ?", dueBefore.UTC()).
		Order("CASE WHEN last_polled_at IS NULL THEN 0 ELSE 1 END, last_polled_at ASC, key ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]schema.Company, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toDomain())
	}
	return out, nil
}

// MarkPolled writes poll bookkeeping for one company.
func (r *Repository) MarkPolled(ctx context.Context, p schema.MarkPolledParams) error {
	if p.Key == "" {
		return fmt.Errorf("company key is required")
	}
	at := p.LastPolledAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	pollState := string(p.PollState)
	if pollState == "" {
		pollState = string(schema.PollStateActive)
	}
	res := r.get().WithContext(ctx).Model(&Company{}).Where("key = ?", p.Key).Updates(map[string]any{
		"last_polled_at":    at.UTC(),
		"board_etag":        p.BoardETag,
		"poll_fail_count":   p.PollFailCount,
		"empty_poll_streak": p.EmptyPollStreak,
		"poll_state":        pollState,
		"updated_at":        time.Now().UTC(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return companies.ErrCompanyNotFound
	}
	return nil
}

// MarkResolution commits a resolve outcome for one company.
func (r *Repository) MarkResolution(ctx context.Context, p schema.MarkResolutionParams) error {
	if p.Key == "" {
		return fmt.Errorf("company key is required")
	}
	if p.ResolutionStatus == "" {
		return fmt.Errorf("resolution status is required")
	}
	res := r.get().WithContext(ctx).Model(&Company{}).Where("key = ?", p.Key).Updates(map[string]any{
		"resolution_status": string(p.ResolutionStatus),
		"resolution_method": string(p.ResolutionMethod),
		"ats":               string(p.ATS),
		"ats_slug":          p.ATSSlug,
		"board_url":         p.BoardURL,
		"next_resolve_at":   p.NextResolveAt,
		"updated_at":        time.Now().UTC(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return companies.ErrCompanyNotFound
	}
	return nil
}
