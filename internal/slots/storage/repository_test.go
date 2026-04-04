package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testSlotsDB(t *testing.T) *gorm.DB {
	t.Helper()
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		`CREATE TABLE slots (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
	} {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestRepository_slotCRUD(t *testing.T) {
	ctx := context.Background()
	db := testSlotsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	if n, err := repo.Count(ctx); err != nil || n != 0 {
		t.Fatalf("count: n=%d err=%v", n, err)
	}
	if err := repo.Create(ctx, id, "alpha"); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.Count(ctx); err != nil || n != 1 {
		t.Fatalf("count after create: n=%d err=%v", n, err)
	}

	got, err := repo.GetByID(ctx, id.String())
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "alpha" {
		t.Fatalf("name %q", got.Name)
	}

	rows, err := repo.List(ctx)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list: %+v err=%v", rows, err)
	}

	if err := repo.Delete(ctx, id.String()); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, id.String()); err != slots.ErrNotFound {
		t.Fatalf("second delete: %v", err)
	}
}

func TestRepository_GetByID_notFound(t *testing.T) {
	ctx := context.Background()
	db := testSlotsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	_, err := repo.GetByID(ctx, uuid.MustParse("22222222-2222-4222-8222-222222222222").String())
	if err != slots.ErrNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestRepository_List_order(t *testing.T) {
	ctx := context.Background()
	db := testSlotsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	first := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	second := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	if err := repo.Create(ctx, first, "older"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := repo.Create(ctx, second, "newer"); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != first.String() || rows[1].ID != second.String() {
		t.Fatalf("order: %+v", rows)
	}
}
