package ingest

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testWatermarkDB(t *testing.T) *gorm.DB {
	t.Helper()
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		CREATE TABLE ingest_watermarks (
			source_id TEXT PRIMARY KEY,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL
		)
	`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestGormWatermarkStore_roundTrip(t *testing.T) {
	ctx := context.Background()
	db := testWatermarkDB(t)
	s := NewGormWatermarkStore(pgsql.NewGetter(db))

	got, err := s.GetCursor(ctx, "Europe_Remotely")
	if err != nil || got != "" {
		t.Fatalf("GetCursor empty = (%q, %v), want (\"\", nil)", got, err)
	}

	if err := s.SetCursor(ctx, "europe_remotely", "opaque-1"); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetCursor(ctx, "europe_remotely")
	if err != nil || got != "opaque-1" {
		t.Fatalf("GetCursor = (%q, %v), want (opaque-1, nil)", got, err)
	}

	if err := s.SetCursor(ctx, "europe_remotely", ""); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetCursor(ctx, "europe_remotely")
	if err != nil || got != "" {
		t.Fatalf("after clear GetCursor = (%q, %v), want (\"\", nil)", got, err)
	}
}
