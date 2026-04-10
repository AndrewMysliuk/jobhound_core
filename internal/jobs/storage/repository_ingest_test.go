package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	jobsschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testJobsDB(t *testing.T) *gorm.DB {
	t.Helper()
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	stmts := []string{
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
			timezone_offsets TEXT NOT NULL DEFAULT '[]',
			position TEXT,
			user_id TEXT,
			stage1_status TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestRepository_SaveIngest_insertSetsStage1(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	j := jobdata.Job{
		ID: "j1", Source: "src", Title: "t", Company: "c", URL: "https://u",
		Description: "d1", PostedAt: now, Tags: []string{"go", "rust"},
	}

	skipped, err := repo.SaveIngest(ctx, j)
	if err != nil || skipped {
		t.Fatalf("SaveIngest = (%v, %v), want (false, nil)", skipped, err)
	}

	var st *string
	if err := db.Raw(`SELECT stage1_status FROM jobs WHERE id = ?`, "j1").Scan(&st).Error; err != nil {
		t.Fatal(err)
	}
	if st == nil || *st != jobsschema.Stage1StatusPassed {
		t.Fatalf("stage1_status = %v, want PASSED_STAGE_1", st)
	}
}

func TestRepository_SaveIngest_skipUnchanged(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	passed := jobsschema.Stage1StatusPassed
	if err := db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
		VALUES ('j1', 'src', 't', 'c', 'https://u', 'same', '["go"]', ?, ?, ?, ?)`,
		now, passed, now, now).Error; err != nil {
		t.Fatal(err)
	}

	j := jobdata.Job{
		ID: "j1", Source: "src", Title: "t", Company: "c", URL: "https://u",
		Description: "same", PostedAt: now, Tags: []string{"go"},
	}
	skipped, err := repo.SaveIngest(ctx, j)
	if err != nil || !skipped {
		t.Fatalf("SaveIngest = (%v, %v), want (true, nil)", skipped, err)
	}
}

func TestRepository_SaveIngest_descriptionOnlyUpdates(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	passed := jobsschema.Stage1StatusPassed
	if err := db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
		VALUES ('j1', 'src', 't', 'c', 'https://u', 'old', '["go"]', ?, ?, ?, ?)`,
		now, passed, now, now).Error; err != nil {
		t.Fatal(err)
	}

	j := jobdata.Job{
		ID: "j1", Source: "src", Title: "t", Company: "c", URL: "https://u",
		Description: "new", PostedAt: now, Tags: []string{"go"},
	}
	skipped, err := repo.SaveIngest(ctx, j)
	if err != nil || skipped {
		t.Fatalf("SaveIngest = (%v, %v), want (false, nil)", skipped, err)
	}

	var desc string
	if err := db.Raw(`SELECT description FROM jobs WHERE id = ?`, "j1").Scan(&desc).Error; err != nil {
		t.Fatal(err)
	}
	if desc != "new" {
		t.Fatalf("description = %q", desc)
	}
}

func TestRepository_SaveIngest_legacyNullStage1FullUpdate(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	if err := db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
		VALUES ('j1', 'src', 't', 'c', 'https://u', 'same', '["go"]', ?, NULL, ?, ?)`,
		now, now, now).Error; err != nil {
		t.Fatal(err)
	}

	j := jobdata.Job{
		ID: "j1", Source: "src", Title: "t", Company: "c", URL: "https://u",
		Description: "same", PostedAt: now, Tags: []string{"go"},
	}
	skipped, err := repo.SaveIngest(ctx, j)
	if err != nil || skipped {
		t.Fatalf("SaveIngest = (%v, %v), want (false, nil)", skipped, err)
	}

	var st *string
	if err := db.Raw(`SELECT stage1_status FROM jobs WHERE id = ?`, "j1").Scan(&st).Error; err != nil {
		t.Fatal(err)
	}
	if st == nil || *st != jobsschema.Stage1StatusPassed {
		t.Fatalf("stage1_status = %v", st)
	}
}

func TestRepository_SaveIngest_descriptionOnlyDoesNotTouchPipelineRunJobs(t *testing.T) {
	ctx := context.Background()
	db := testJobsDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	passed := jobsschema.Stage1StatusPassed

	extra := []string{
		`CREATE TABLE pipeline_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE pipeline_run_jobs (
			pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			stage2_status TEXT NOT NULL,
			stage3_status TEXT,
			stage3_rationale TEXT,
			PRIMARY KEY (pipeline_run_id, job_id)
		)`,
	}
	for _, s := range extra {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, stage1_status, created_at, updated_at)
		VALUES ('j1', 'src', 't', 'c', 'https://u', 'old', '["go"]', ?, ?, ?, ?)`,
		now, passed, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO pipeline_runs (id, created_at, updated_at) VALUES (1, ?, ?)`, now, now).Error; err != nil {
		t.Fatal(err)
	}
	wantStatus := string(pipeline.RunJobPassedStage2)
	if err := db.Exec(
		`INSERT INTO pipeline_run_jobs (pipeline_run_id, job_id, stage2_status) VALUES (1, 'j1', ?)`,
		wantStatus,
	).Error; err != nil {
		t.Fatal(err)
	}

	j := jobdata.Job{
		ID: "j1", Source: "src", Title: "t", Company: "c", URL: "https://u",
		Description: "new", PostedAt: now, Tags: []string{"go"},
	}
	skipped, err := repo.SaveIngest(ctx, j)
	if err != nil || skipped {
		t.Fatalf("SaveIngest = (%v, %v), want (false, nil)", skipped, err)
	}

	var got string
	if err := db.Raw(
		`SELECT stage2_status FROM pipeline_run_jobs WHERE pipeline_run_id = 1 AND job_id = 'j1'`,
	).Scan(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != wantStatus {
		t.Fatalf("pipeline_run_jobs.stage2_status = %q, want %q", got, wantStatus)
	}
}
