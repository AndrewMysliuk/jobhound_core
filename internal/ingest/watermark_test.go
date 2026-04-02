package ingest

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var testSlotIDW = uuid.MustParse("33333333-3333-4333-8333-333333333333")

func testWatermarkDB(t *testing.T) *gorm.DB {
	t.Helper()
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		CREATE TABLE ingest_watermarks (
			slot_id TEXT NOT NULL,
			source_id TEXT NOT NULL,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (slot_id, source_id)
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

	got, err := s.GetCursor(ctx, testSlotIDW, "Europe_Remotely")
	if err != nil || got != "" {
		t.Fatalf("GetCursor empty = (%q, %v), want (\"\", nil)", got, err)
	}

	if err := s.SetCursor(ctx, testSlotIDW, "europe_remotely", "opaque-1"); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetCursor(ctx, testSlotIDW, "europe_remotely")
	if err != nil || got != "opaque-1" {
		t.Fatalf("GetCursor = (%q, %v), want (opaque-1, nil)", got, err)
	}

	if err := s.SetCursor(ctx, testSlotIDW, "europe_remotely", ""); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetCursor(ctx, testSlotIDW, "europe_remotely")
	if err != nil || got != "" {
		t.Fatalf("after clear GetCursor = (%q, %v), want (\"\", nil)", got, err)
	}
}

func TestGormWatermarkStore_sameSourceDifferentSlots(t *testing.T) {
	ctx := context.Background()
	db := testWatermarkDB(t)
	s := NewGormWatermarkStore(pgsql.NewGetter(db))
	sa := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	sb := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	requireNoErr := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	requireNoErr(s.SetCursor(ctx, sa, "src1", "cursor-a"))
	requireNoErr(s.SetCursor(ctx, sb, "src1", "cursor-b"))
	var ca, cb string
	requireNoErr(db.Raw(`SELECT cursor FROM ingest_watermarks WHERE slot_id = ? AND source_id = ?`, sa.String(), "src1").Scan(&ca).Error)
	requireNoErr(db.Raw(`SELECT cursor FROM ingest_watermarks WHERE slot_id = ? AND source_id = ?`, sb.String(), "src1").Scan(&cb).Error)
	if ca != "cursor-a" || cb != "cursor-b" {
		t.Fatalf("cursors: %q %q", ca, cb)
	}
}
