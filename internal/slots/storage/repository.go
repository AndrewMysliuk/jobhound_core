package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

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
	// createMu serializes CreateWithIdempotency (slot cap + idempotency mapping) for single-process API.
	createMu sync.Mutex
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

// CreateWithIdempotency inserts a slot and idempotency mapping, or returns an existing slot when the key
// matches the same name (replay). replay is true when no new slot row was inserted.
func (r *Repository) CreateWithIdempotency(ctx context.Context, key uuid.UUID, name string) (Slot, bool, error) {
	r.createMu.Lock()
	defer r.createMu.Unlock()
	var out Slot
	var replay bool
	err := r.get().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var idemp SlotIdempotency
		err := tx.Where("idempotency_key = ?", key.String()).First(&idemp).Error
		if err == nil {
			var s Slot
			if err := tx.Where("id = ?", idemp.SlotID).First(&s).Error; err != nil {
				return err
			}
			if s.Name != name {
				return slots.ErrIdempotencyKeyConflict
			}
			out = s
			replay = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var n int64
		if err := tx.Model(&Slot{}).Count(&n).Error; err != nil {
			return err
		}
		if n >= MaxSlots {
			return slots.ErrSlotLimitReached
		}
		id := uuid.New()
		slotRow := Slot{ID: id.String(), Name: name}
		if err := tx.Create(&slotRow).Error; err != nil {
			return err
		}
		if err := tx.Create(&SlotIdempotency{IdempotencyKey: key.String(), SlotID: id.String()}).Error; err != nil {
			return err
		}
		out = slotRow
		replay = false
		return nil
	})
	return out, replay, err
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
