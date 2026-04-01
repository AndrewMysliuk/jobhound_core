package ingest

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WatermarkStore persists per-source opaque cursors in ingest_watermarks (006 contract).
type WatermarkStore interface {
	// GetCursor returns the stored cursor, or empty if missing or NULL.
	GetCursor(ctx context.Context, sourceID string) (cursor string, err error)
	// SetCursor upserts the row; empty cursor is stored as SQL NULL.
	SetCursor(ctx context.Context, sourceID string, cursor string) error
}

type ingestWatermarkRow struct {
	SourceID  string    `gorm:"column:source_id;primaryKey"`
	Cursor    *string   `gorm:"column:cursor"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (ingestWatermarkRow) TableName() string { return "ingest_watermarks" }

// GormWatermarkStore implements [WatermarkStore] via GORM.
type GormWatermarkStore struct {
	get pgsql.GormGetter
}

// NewGormWatermarkStore wires Postgres/SQLite-backed watermark persistence.
func NewGormWatermarkStore(get pgsql.GormGetter) *GormWatermarkStore {
	return &GormWatermarkStore{get: get}
}

// GetCursor implements [WatermarkStore].
func (s *GormWatermarkStore) GetCursor(ctx context.Context, sourceID string) (string, error) {
	if s == nil || s.get == nil {
		return "", errors.New("ingest: nil GormWatermarkStore")
	}
	id := NormalizeSourceID(sourceID)
	if id == "" {
		return "", nil
	}
	var row ingestWatermarkRow
	err := s.get().WithContext(ctx).Where("source_id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if row.Cursor == nil {
		return "", nil
	}
	return *row.Cursor, nil
}

// SetCursor implements [WatermarkStore].
func (s *GormWatermarkStore) SetCursor(ctx context.Context, sourceID string, cursor string) error {
	if s == nil || s.get == nil {
		return errors.New("ingest: nil GormWatermarkStore")
	}
	id := NormalizeSourceID(sourceID)
	if id == "" {
		return nil
	}
	now := time.Now().UTC()
	var cur *string
	if strings.TrimSpace(cursor) != "" {
		c := strings.TrimSpace(cursor)
		cur = &c
	}
	row := ingestWatermarkRow{SourceID: id, Cursor: cur, UpdatedAt: now}
	return s.get().WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"cursor", "updated_at"}),
	}).Create(&row).Error
}
