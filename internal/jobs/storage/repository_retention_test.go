package storage

import (
	"context"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	jobutils "github.com/andrewmysliuk/jobhound_core/internal/jobs/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"gorm.io/gorm"
)

func testJobsDBWithMatchFK(t *testing.T) *gorm.DB {
	t.Helper()
	db := testJobsDB(t)
	stmts := []string{
		`CREATE TABLE profile_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TIMESTAMP NOT NULL,
			finished_at TIMESTAMP,
			jobs_scored INTEGER NOT NULL DEFAULT 0,
			sources_skipped INTEGER NOT NULL DEFAULT 0,
			idempotency_key TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE profile_matches (
			profile_id TEXT NOT NULL,
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			bucket TEXT NOT NULL CHECK (bucket IN ('PASSED', 'REJECTED')),
			score INTEGER NOT NULL,
			signals TEXT NOT NULL,
			user_status TEXT NOT NULL DEFAULT 'NEW' CHECK (user_status IN ('NEW', 'HIDDEN')),
			run_id INTEGER NOT NULL REFERENCES profile_runs(id),
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (profile_id, job_id)
		)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestRepository_DeleteJobsLastSeenBeforeUTC(t *testing.T) {
	ctx := context.Background()
	db := testJobsDBWithMatchFK(t)
	repo := NewRepository(pgsql.NewGetter(db))

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	cutoff := jobutils.CutoffUTC(now, config.Config{})
	staleSeen := now.Add(-31 * 24 * time.Hour)
	freshSeen := now.Add(-1 * 24 * time.Hour)
	oldCreated := now.Add(-100 * 24 * time.Hour)

	if err := db.Exec(`
		INSERT INTO jobs (id, title, company, url, description, tags, first_seen_at, last_seen_at, created_at, updated_at)
		VALUES ('stale', 't', 'c', 'https://u', 'd', '[]', ?, ?, ?, ?)`,
		staleSeen, staleSeen, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO jobs (id, title, company, url, description, tags, first_seen_at, last_seen_at, created_at, updated_at)
		VALUES ('fresh', 't', 'c', 'https://u', 'd', '[]', ?, ?, ?, ?)`,
		oldCreated, freshSeen, oldCreated, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO profile_runs (id, profile_id, status, started_at, idempotency_key)
		VALUES (1, 'andrew', 'SUCCEEDED', ?, '11111111-1111-1111-1111-111111111111')`, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO profile_matches (profile_id, job_id, bucket, score, signals, run_id, updated_at)
		VALUES ('andrew', 'stale', 'PASSED', 1, '[]', 1, ?)`, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO profile_matches (profile_id, job_id, bucket, score, signals, run_id, updated_at)
		VALUES ('andrew', 'fresh', 'PASSED', 1, '[]', 1, ?)`, now).Error; err != nil {
		t.Fatal(err)
	}

	n, err := repo.DeleteJobsLastSeenBeforeUTC(ctx, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted = %d, want 1", n)
	}

	var cnt int64
	if err := db.Raw(`SELECT COUNT(*) FROM jobs WHERE id = 'stale'`).Scan(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatal("job last seen 31 days ago should be deleted")
	}
	if err := db.Raw(`SELECT COUNT(*) FROM profile_matches WHERE job_id = 'stale'`).Scan(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("profile_matches for stale should CASCADE-delete, got count %d", cnt)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM jobs WHERE id = 'fresh'`).Scan(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("job last seen inside the window should remain, count=%d", cnt)
	}
	if err := db.Raw(`SELECT COUNT(*) FROM profile_matches WHERE job_id = 'fresh'`).Scan(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("profile_matches for fresh should remain, count=%d", cnt)
	}
}
