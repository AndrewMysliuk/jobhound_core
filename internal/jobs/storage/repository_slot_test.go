package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testSlotJobsDB(t *testing.T) *gorm.DB {
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
		`CREATE TABLE jobs (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			company TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			apply_url TEXT,
			description TEXT NOT NULL DEFAULT '',
			posted_at TIMESTAMP,
			is_remote INTEGER,
			country_code TEXT NOT NULL DEFAULT '',
			salary_raw TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			position TEXT,
			user_id TEXT,
			stage1_status TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE slot_jobs (
			slot_id TEXT NOT NULL,
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			first_seen_at TIMESTAMP NOT NULL,
			PRIMARY KEY (slot_id, job_id)
		)`,
		`CREATE TABLE pipeline_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP NOT NULL,
			slot_id TEXT,
			broad_filter_key_hash TEXT
		)`,
		`CREATE TABLE pipeline_run_jobs (
			pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			PRIMARY KEY (pipeline_run_id, job_id)
		)`,
	} {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestRepository_slotQueries_tableDriven(t *testing.T) {
	ctx := context.Background()
	slotA := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	slotB := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	passed := jobs.Stage1StatusPassed

	t.Run("empty_slot_returns_no_jobs", func(t *testing.T) {
		db := testSlotJobsDB(t)
		repo := NewRepository(pgsql.NewGetter(db))
		got, err := repo.ListSlotJobsPassedStage1(ctx, slotA)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("len = %d, want 0", len(got))
		}
	})

	t.Run("lists_only_slot_and_passed_stage1", func(t *testing.T) {
		db := testSlotJobsDB(t)
		repo := NewRepository(pgsql.NewGetter(db))
		// j1: slot A, passed — expect in list for A
		if err := db.Exec(`
			INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
			VALUES ('j1', 's', 't', 'c', 'u', 'd', '[]', ?, ?, ?, ?)`,
			now, passed, now, now).Error; err != nil {
			t.Fatal(err)
		}
		// j2: other slot B — same stage1, linked only to B
		if err := db.Exec(`
			INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
			VALUES ('j2', 's', 't', 'c', 'u', 'd', '[]', ?, ?, ?, ?)`,
			now, passed, now, now).Error; err != nil {
			t.Fatal(err)
		}
		// j3: in slot A but not passed stage 1
		if err := db.Exec(`
			INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
			VALUES ('j3', 's', 't', 'c', 'u', 'd', '[]', ?, NULL, ?, ?)`,
			now, now, now).Error; err != nil {
			t.Fatal(err)
		}
		for _, row := range []struct {
			slot uuid.UUID
			job  string
		}{
			{slotA, "j1"},
			{slotB, "j2"},
			{slotA, "j3"},
		} {
			if err := repo.UpsertSlotJob(ctx, row.slot, row.job); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.ListSlotJobsPassedStage1(ctx, slotA)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID != "j1" {
			t.Fatalf("got %+v, want single job j1", got)
		}
	})

	t.Run("upsert_pair_idempotent", func(t *testing.T) {
		db := testSlotJobsDB(t)
		repo := NewRepository(pgsql.NewGetter(db))
		if err := db.Exec(`
			INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
			VALUES ('jx', 's', 't', 'c', 'u', 'd', '[]', ?, ?, ?, ?)`,
			now, passed, now, now).Error; err != nil {
			t.Fatal(err)
		}
		if err := repo.UpsertSlotJob(ctx, slotA, "jx"); err != nil {
			t.Fatal(err)
		}
		if err := repo.UpsertSlotJob(ctx, slotA, "jx"); err != nil {
			t.Fatal(err)
		}
		var n int64
		if err := db.Raw(`SELECT COUNT(*) FROM slot_jobs WHERE slot_id = ? AND job_id = 'jx'`, slotA.String()).Scan(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("count = %d, want 1", n)
		}
	})

	t.Run("upsert_missing_job_fk", func(t *testing.T) {
		db := testSlotJobsDB(t)
		repo := NewRepository(pgsql.NewGetter(db))
		err := repo.UpsertSlotJob(ctx, slotA, "nope")
		if err == nil {
			t.Fatal("expected error for missing job row")
		}
	})

	t.Run("passed_stage2_run_ordered_by_posted_at_desc_nulls_last", func(t *testing.T) {
		db := testSlotJobsDB(t)
		repo := NewRepository(pgsql.NewGetter(db))
		t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		t2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		for _, row := range []struct {
			id       string
			postedAt *time.Time
		}{
			{"early", &t1},
			{"late", &t2},
			{"nilpa", nil},
		} {
			if err := db.Exec(`
				INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
				VALUES (?, 's', 't', 'c', 'u', 'd', '[]', ?, ?, ?, ?)`,
				row.id, row.postedAt, passed, now, now).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := db.Exec(`INSERT INTO pipeline_runs (id, created_at) VALUES (1, ?)`, now).Error; err != nil {
			t.Fatal(err)
		}
		st := string(pipeline.RunJobPassedStage2)
		for _, id := range []string{"early", "late", "nilpa"} {
			if err := db.Exec(
				`INSERT INTO pipeline_run_jobs (pipeline_run_id, job_id, status) VALUES (1, ?, ?)`,
				id, st,
			).Error; err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.ListPassedStage2JobsForRun(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("len = %d", len(got))
		}
		if got[0].ID != "late" || got[1].ID != "early" || got[2].ID != "nilpa" {
			t.Fatalf("order = %v %v %v, want late, early, nilpa", got[0].ID, got[1].ID, got[2].ID)
		}
	})
}

func TestRepository_UpsertSlotJob_requiresIDs(t *testing.T) {
	ctx := context.Background()
	db := testSlotJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	if err := repo.UpsertSlotJob(ctx, uuid.Nil, "x"); err == nil {
		t.Fatal("expected error for nil slot")
	}
	slot := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	if err := repo.UpsertSlotJob(ctx, slot, ""); err == nil {
		t.Fatal("expected error for empty job id")
	}
}

func TestRepository_ListPassedStage2JobsForRun_requiresRunID(t *testing.T) {
	ctx := context.Background()
	db := testSlotJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	if _, err := repo.ListPassedStage2JobsForRun(ctx, 0); err == nil {
		t.Fatal("expected error")
	}
}
