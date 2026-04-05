package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
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
			stage3_rationale TEXT,
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
	passed := schema.Stage1StatusPassed

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

func TestRepository_ListSlotStage1Jobs_paginationAndTotal(t *testing.T) {
	ctx := context.Background()
	slotA := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	passed := schema.Stage1StatusPassed
	db := testSlotJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	tA := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	tB := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		id   string
		post *time.Time
	}{
		{"ja", &tA},
		{"jb", &tB},
		{"jc", &tA},
	} {
		if err := db.Exec(`
			INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
			VALUES (?, 's', 't', 'c', 'u', 'd', '[]', ?, ?, ?, ?)`,
			row.id, row.post, passed, now, now).Error; err != nil {
			t.Fatal(err)
		}
		if err := repo.UpsertSlotJob(ctx, slotA, row.id); err != nil {
			t.Fatal(err)
		}
	}
	got, total, err := repo.ListSlotStage1Jobs(ctx, slotA, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total %d want 3", total)
	}
	if len(got) != 2 {
		t.Fatalf("len %d want 2", len(got))
	}
	// posted_at DESC: jb (May) first; then March tie-break id ASC → ja, jc
	if got[0].Job.ID != "jb" || got[1].Job.ID != "ja" {
		t.Fatalf("order got %q %q want jb ja", got[0].Job.ID, got[1].Job.ID)
	}
	got2, _, err := repo.ListSlotStage1Jobs(ctx, slotA, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 1 || got2[0].Job.ID != "jc" {
		t.Fatalf("page2 got %+v", got2)
	}
}

func TestRepository_ListPipelineRunStage2Jobs_bucketFilter(t *testing.T) {
	ctx := context.Background()
	slotA := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	passed := schema.Stage1StatusPassed
	db := testSlotJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	if err := db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
		VALUES ('jp', 's', 't', 'c', 'u', 'd', '[]', ?, ?, ?, ?)`,
		now, passed, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
		VALUES ('jr', 's', 't', 'c', 'u', 'd', '[]', ?, ?, ?, ?)`,
		now, passed, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertSlotJob(ctx, slotA, "jp"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertSlotJob(ctx, slotA, "jr"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO pipeline_runs (id, created_at, slot_id) VALUES (1, ?, ?)`, now, slotA.String()).Error; err != nil {
		t.Fatal(err)
	}
	stP := string(pipeline.RunJobPassedStage2)
	stR := string(pipeline.RunJobRejectedStage2)
	if err := db.Exec(`INSERT INTO pipeline_run_jobs (pipeline_run_id, job_id, status) VALUES (1, 'jp', ?), (1, 'jr', ?)`, stP, stR).Error; err != nil {
		t.Fatal(err)
	}
	all, total, err := repo.ListPipelineRunStage2Jobs(ctx, slotA, 1, schema.ListBucketAll, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(all) != 2 {
		t.Fatalf("all: total=%d len=%d", total, len(all))
	}
	passedOnly, totalP, err := repo.ListPipelineRunStage2Jobs(ctx, slotA, 1, schema.ListBucketPassed, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if totalP != 1 || len(passedOnly) != 1 || passedOnly[0].Job.ID != "jp" {
		t.Fatalf("passed: %+v", passedOnly)
	}
	failedOnly, totalF, err := repo.ListPipelineRunStage2Jobs(ctx, slotA, 1, schema.ListBucketFailed, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if totalF != 1 || len(failedOnly) != 1 || failedOnly[0].Job.ID != "jr" {
		t.Fatalf("failed: %+v", failedOnly)
	}
}
