package storage

import (
	"context"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/gorm"
)

func testJobsDBWithPipelineFK(t *testing.T) *gorm.DB {
	t.Helper()
	db := testJobsDB(t)
	stmts := []string{
		`CREATE TABLE pipeline_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE pipeline_run_jobs (
			pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			PRIMARY KEY (pipeline_run_id, job_id)
		)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestRepository_DeleteJobsCreatedBeforeUTC_cascadesPipelineRunJobs(t *testing.T) {
	ctx := context.Background()
	db := testJobsDBWithPipelineFK(t)
	repo := NewRepository(pgsql.NewGetter(db))

	old := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	if err := db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, created_at, updated_at)
		VALUES ('oldjob', 's', 't', 'c', 'https://u', 'd', '[]', ?, ?)`,
		old, old).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, created_at, updated_at)
		VALUES ('newjob', 's', 't', 'c', 'https://u', 'd', '[]', ?, ?)`,
		newer, newer).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO pipeline_runs (id) VALUES (1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO pipeline_run_jobs (pipeline_run_id, job_id, status) VALUES (1, 'oldjob', 'PASSED_STAGE_2')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO pipeline_run_jobs (pipeline_run_id, job_id, status) VALUES (1, 'newjob', 'PASSED_STAGE_2')`).Error; err != nil {
		t.Fatal(err)
	}

	cutoff := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	n, err := repo.DeleteJobsCreatedBeforeUTC(ctx, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted = %d, want 1", n)
	}

	var cnt int64
	if err := db.Raw(`SELECT COUNT(*) FROM jobs WHERE id = 'oldjob'`).Scan(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("old job still present")
	}
	if err := db.Raw(`SELECT COUNT(*) FROM pipeline_run_jobs WHERE job_id = 'oldjob'`).Scan(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("pipeline_run_jobs for oldjob should CASCADE-delete, got count %d", cnt)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM jobs WHERE id = 'newjob'`).Scan(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("newjob should remain, count=%d", cnt)
	}
}
