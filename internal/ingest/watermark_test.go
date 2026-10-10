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
			source_id TEXT NOT NULL,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (source_id)
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

func TestGormWatermarkStore_sourcesAreIndependent(t *testing.T) {
	ctx := context.Background()
	db := testWatermarkDB(t)
	s := NewGormWatermarkStore(pgsql.NewGetter(db))
	requireNoErr := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	requireNoErr(s.SetCursor(ctx, "src1", "cursor-a"))
	requireNoErr(s.SetCursor(ctx, "src2", "cursor-b"))
	requireNoErr(s.SetCursor(ctx, "src1", "cursor-c"))
	var ca, cb string
	requireNoErr(db.Raw(`SELECT cursor FROM ingest_watermarks WHERE source_id = ?`, "src1").Scan(&ca).Error)
	requireNoErr(db.Raw(`SELECT cursor FROM ingest_watermarks WHERE source_id = ?`, "src2").Scan(&cb).Error)
	if ca != "cursor-c" || cb != "cursor-b" {
		t.Fatalf("cursors: %q %q", ca, cb)
	}
}
