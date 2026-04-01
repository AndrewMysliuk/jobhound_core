package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testDB builds an in-memory SQLite schema compatible with 007 migrations (FKs + composite PK).
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Unique in-memory DB name so parallel/subtests do not share one SQLite catalog.
	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

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
			position TEXT,
			user_id TEXT,
			stage1_status TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE pipeline_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP NOT NULL,
			broad_filter_key_hash TEXT
		)`,
		`CREATE TABLE pipeline_run_jobs (
			pipeline_run_id INTEGER NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			PRIMARY KEY (pipeline_run_id, job_id)
		)`,
	}
	for _, s := range stmts {
		require.NoError(t, db.Exec(s).Error)
	}
	return db
}

func seedJob(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	now := time.Now().UTC()
	err := db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, created_at, updated_at)
		VALUES (?, 'src', 't', 'c', 'https://x', 'd', '[]', ?, ?)`,
		id, now, now).Error
	require.NoError(t, err)
}

func TestRepository_SetBroadFilterKeyHash(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	runID, err := repo.CreateRun(ctx)
	require.NoError(t, err)

	want := "ab" + strings.Repeat("0", 62)
	require.NoError(t, repo.SetBroadFilterKeyHash(ctx, runID, want))

	var got *string
	require.NoError(t, db.Raw(`SELECT broad_filter_key_hash FROM pipeline_runs WHERE id = ?`, runID).Scan(&got).Error)
	require.NotNil(t, got)
	require.Equal(t, want, *got)

	require.NoError(t, repo.SetBroadFilterKeyHash(ctx, runID, "  "))
	var after string
	require.NoError(t, db.Raw(`SELECT broad_filter_key_hash FROM pipeline_runs WHERE id = ?`, runID).Scan(&after).Error)
	require.Equal(t, want, after, "empty hash must not clear column")
}

func TestRepository_CreateRun(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	id1, err := repo.CreateRun(ctx)
	require.NoError(t, err)
	require.Positive(t, id1)

	id2, err := repo.CreateRun(ctx)
	require.NoError(t, err)
	require.Greater(t, id2, id1)
}

func TestRepository_SetRunJobStatus_stage2Then3(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	runID, err := repo.CreateRun(ctx)
	require.NoError(t, err)
	seedJob(t, db, "job-a")

	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "job-a", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "job-a", pipeline.RunJobPassedStage3))

	var status string
	require.NoError(t, db.Raw(
		`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = ?`,
		runID, "job-a").Scan(&status).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage3), status)
}

func TestRepository_SetRunJobStatus_invalidTransition(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	runID, err := repo.CreateRun(ctx)
	require.NoError(t, err)
	seedJob(t, db, "job-x")

	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "job-x", pipeline.RunJobRejectedStage2))
	err = repo.SetRunJobStatus(ctx, runID, "job-x", pipeline.RunJobPassedStage3)
	require.ErrorIs(t, err, ErrInvalidRunJobTransition)
}

func TestRepository_SetRunJobStatus_idempotent(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	runID, err := repo.CreateRun(ctx)
	require.NoError(t, err)
	seedJob(t, db, "job-y")

	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "job-y", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "job-y", pipeline.RunJobPassedStage2))
}

func TestRepository_SetRunJobStatus_insertRequiresStage2Outcome(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	runID, err := repo.CreateRun(ctx)
	require.NoError(t, err)
	seedJob(t, db, "job-z")

	err = repo.SetRunJobStatus(ctx, runID, "job-z", pipeline.RunJobPassedStage3)
	require.ErrorIs(t, err, ErrInvalidRunJobTransition)
}

func TestRepository_ListPassedStage2JobIDs_orderAndFilter(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	runID, err := repo.CreateRun(ctx)
	require.NoError(t, err)
	for _, id := range []string{"m", "a", "z"} {
		seedJob(t, db, id)
		require.NoError(t, repo.SetRunJobStatus(ctx, runID, id, pipeline.RunJobPassedStage2))
	}
	seedJob(t, db, "rej")
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "rej", pipeline.RunJobRejectedStage2))

	got, err := repo.ListPassedStage2JobIDs(ctx, runID)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "m", "z"}, got)
}

func TestRepository_SetRunJobStatus_emptyJobID(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	err := repo.SetRunJobStatus(context.Background(), 1, "", pipeline.RunJobPassedStage2)
	require.Error(t, err)
}

func TestRepository_SetRunJobStatus_invalidStatusString(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	err := repo.SetRunJobStatus(context.Background(), 1, "j", pipeline.RunJobStatus("bogus"))
	require.ErrorIs(t, err, ErrInvalidRunJobStatus)
}
