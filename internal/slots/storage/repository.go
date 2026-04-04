package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MaxSlots is the MVP cap (single implicit user).
const MaxSlots = 3

// Repository persists slots.
type Repository struct {
	get pgsql.GormGetter
}

// NewRepository wires slot persistence.
func NewRepository(get pgsql.GormGetter) *Repository {
	return &Repository{get: get}
}

// Count returns the number of slot rows.
func (r *Repository) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.get().WithContext(ctx).Model(&Slot{}).Count(&n).Error
	return n, err
}

// List returns all slots ordered by created_at ascending (stable cap ≤ 3).
func (r *Repository) List(ctx context.Context) ([]Slot, error) {
	var rows []Slot
	err := r.get().WithContext(ctx).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

// GetByID loads a slot by UUID string or returns [slots.ErrNotFound].
func (r *Repository) GetByID(ctx context.Context, id string) (Slot, error) {
	u, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return Slot{}, slots.ErrNotFound
	}
	var row Slot
	err = r.get().WithContext(ctx).Where("id = ?", u.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Slot{}, slots.ErrNotFound
	}
	if err != nil {
		return Slot{}, err
	}
	return row, nil
}

// Create inserts a slot with the given id and name.
func (r *Repository) Create(ctx context.Context, id uuid.UUID, name string) error {
	if id == uuid.Nil {
		return fmt.Errorf("slot id is required")
	}
	row := Slot{
		ID:   id.String(),
		Name: name,
	}
	return r.get().WithContext(ctx).Create(&row).Error
}

// Delete removes a slot by id or returns [slots.ErrNotFound].
func (r *Repository) Delete(ctx context.Context, id string) error {
	u, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return slots.ErrNotFound
	}
	res := r.get().WithContext(ctx).Where("id = ?", u.String()).Delete(&Slot{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return slots.ErrNotFound
	}
	return nil
}
