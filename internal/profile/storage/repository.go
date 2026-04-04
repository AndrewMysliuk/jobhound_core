package storage

import (
	"context"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
)

const singletonProfileID = 1

// Repository persists the global user_profile row.
type Repository struct {
	get pgsql.GormGetter
}

// NewRepository wires profile persistence.
func NewRepository(get pgsql.GormGetter) *Repository {
	return &Repository{get: get}
}

// Get returns the singleton profile row.
func (r *Repository) Get(ctx context.Context) (UserProfile, error) {
	var row UserProfile
	err := r.get().WithContext(ctx).Where("id = ?", singletonProfileID).First(&row).Error
	return row, err
}

// Set replaces profile text and bumps updated_at.
func (r *Repository) Set(ctx context.Context, text string) (time.Time, error) {
	now := time.Now().UTC()
	row := UserProfile{ID: singletonProfileID, Text: text, UpdatedAt: now}
	err := r.get().WithContext(ctx).Save(&row).Error
	if err != nil {
		return time.Time{}, err
	}
	return now, nil
}
