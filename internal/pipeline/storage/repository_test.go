package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/google/uuid"
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
			slot_id TEXT,
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

	runID, err := repo.CreateRun(ctx, nil)
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

	id1, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	require.Positive(t, id1)

	id2, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	require.Greater(t, id2, id1)

	slot := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	id3, err := repo.CreateRun(ctx, &slot)
	require.NoError(t, err)
	require.Greater(t, id3, id2)
	var gotSlot *string
	require.NoError(t, db.Raw(`SELECT slot_id FROM pipeline_runs WHERE id = ?`, id3).Scan(&gotSlot).Error)
	require.NotNil(t, gotSlot)
	require.Equal(t, slot.String(), *gotSlot)
}

func TestRepository_SetRunJobStatus_stage2Then3(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	runID, err := repo.CreateRun(ctx, nil)
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

	runID, err := repo.CreateRun(ctx, nil)
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

	runID, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	seedJob(t, db, "job-y")

	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "job-y", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "job-y", pipeline.RunJobPassedStage2))
}

func TestRepository_SetRunJobStatus_insertRequiresStage2Outcome(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	runID, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	seedJob(t, db, "job-z")

	err = repo.SetRunJobStatus(ctx, runID, "job-z", pipeline.RunJobPassedStage3)
	require.ErrorIs(t, err, ErrInvalidRunJobTransition)
}

func TestRepository_ListPassedStage2JobIDs_orderAndFilter(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	runID, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	for _, id := range []string{"m", "a", "z"} {
		seedJob(t, db, id)
		require.NoError(t, repo.SetRunJobStatus(ctx, runID, id, pipeline.RunJobPassedStage2))
	}
	seedJob(t, db, "rej")
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "rej", pipeline.RunJobRejectedStage2))

	// posted_at DESC (008); m newest, z middle, a oldest
	tm := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	tz := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	ta := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Exec(`UPDATE jobs SET posted_at = ? WHERE id = 'm'`, tm).Error)
	require.NoError(t, db.Exec(`UPDATE jobs SET posted_at = ? WHERE id = 'z'`, tz).Error)
	require.NoError(t, db.Exec(`UPDATE jobs SET posted_at = ? WHERE id = 'a'`, ta).Error)

	got, err := repo.ListPassedStage2JobIDs(ctx, runID)
	require.NoError(t, err)
	require.Equal(t, []string{"m", "z", "a"}, got)
}

// 007 contract §2: eligible pool is PASSED_STAGE_2 only; terminal stage-3 rows must not appear (same row loses PASSED_STAGE_2 after transition).
func TestRepository_ListPassedStage2JobIDs_excludesAfterStage3(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()
	runID, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	seedJob(t, db, "j")
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "j", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "j", pipeline.RunJobPassedStage3))
	got, err := repo.ListPassedStage2JobIDs(ctx, runID)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestRepository_SetRunJobStatus_terminal3IgnoresStage2Rewrite(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()
	runID, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	seedJob(t, db, "j")
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "j", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "j", pipeline.RunJobPassedStage3))
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "j", pipeline.RunJobPassedStage2))
	var st string
	require.NoError(t, db.Raw(
		`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = ?`,
		runID, "j").Scan(&st).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage3), st)
}

func TestRepository_GetRunJobStatus(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()
	runID, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	seedJob(t, db, "x")
	st, ok, err := repo.GetRunJobStatus(ctx, runID, "x")
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, pipeline.RunJobStatus(""), st)

	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "x", pipeline.RunJobPassedStage2))
	st, ok, err = repo.GetRunJobStatus(ctx, runID, "x")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, pipeline.RunJobPassedStage2, st)
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

func TestRepository_InvalidateStage3SnapshotsForSlot(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	slotA := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	slotB := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")

	runA, err := repo.CreateRun(ctx, &slotA)
	require.NoError(t, err)
	runB, err := repo.CreateRun(ctx, &slotB)
	require.NoError(t, err)
	runLegacy, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)

	for _, id := range []string{"j1", "j2", "j3", "j4", "j5", "j6"} {
		seedJob(t, db, id)
	}

	require.NoError(t, repo.SetRunJobStatus(ctx, runA, "j1", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runA, "j1", pipeline.RunJobPassedStage3))
	require.NoError(t, repo.SetRunJobStatus(ctx, runA, "j2", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runA, "j2", pipeline.RunJobRejectedStage3))
	require.NoError(t, repo.SetRunJobStatus(ctx, runA, "j3", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runA, "j4", pipeline.RunJobRejectedStage2))

	require.NoError(t, repo.SetRunJobStatus(ctx, runB, "j5", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runB, "j5", pipeline.RunJobPassedStage3))

	require.NoError(t, repo.SetRunJobStatus(ctx, runLegacy, "j6", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runLegacy, "j6", pipeline.RunJobRejectedStage3))

	n, err := repo.InvalidateStage3SnapshotsForSlot(ctx, slotA)
	require.NoError(t, err)
	require.Equal(t, int64(2), n)

	var st1, st2, st3, st4 string
	require.NoError(t, db.Raw(`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'j1'`, runA).Scan(&st1).Error)
	require.NoError(t, db.Raw(`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'j2'`, runA).Scan(&st2).Error)
	require.NoError(t, db.Raw(`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'j3'`, runA).Scan(&st3).Error)
	require.NoError(t, db.Raw(`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'j4'`, runA).Scan(&st4).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage2), st1)
	require.Equal(t, string(pipeline.RunJobPassedStage2), st2)
	require.Equal(t, string(pipeline.RunJobPassedStage2), st3)
	require.Equal(t, string(pipeline.RunJobRejectedStage2), st4)

	var stB string
	require.NoError(t, db.Raw(`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'j5'`, runB).Scan(&stB).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage3), stB)

	var stLegacy string
	require.NoError(t, db.Raw(`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'j6'`, runLegacy).Scan(&stLegacy).Error)
	require.Equal(t, string(pipeline.RunJobRejectedStage3), stLegacy)

	_, err = repo.InvalidateStage3SnapshotsForSlot(ctx, uuid.Nil)
	require.Error(t, err)
}

func TestRepository_InvalidateStage2And3SnapshotsForSlot(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	slotA := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	slotB := uuid.MustParse("dddddddd-dddd-4ddd-8ddd-dddddddddddd")

	r1, err := repo.CreateRun(ctx, &slotA)
	require.NoError(t, err)
	r2, err := repo.CreateRun(ctx, &slotA)
	require.NoError(t, err)
	rOther, err := repo.CreateRun(ctx, &slotB)
	require.NoError(t, err)

	seedJob(t, db, "p1")
	seedJob(t, db, "p2")
	seedJob(t, db, "p3")
	require.NoError(t, repo.SetRunJobStatus(ctx, r1, "p1", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, r2, "p2", pipeline.RunJobRejectedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, rOther, "p3", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, rOther, "p3", pipeline.RunJobPassedStage3))

	del, err := repo.InvalidateStage2And3SnapshotsForSlot(ctx, slotA)
	require.NoError(t, err)
	require.Equal(t, int64(2), del)

	var cntA int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pipeline_runs WHERE slot_id = ?`, slotA.String()).Scan(&cntA).Error)
	require.Zero(t, cntA)

	var cntJobs int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pipeline_run_jobs WHERE pipeline_run_id IN (?, ?)`, r1, r2).Scan(&cntJobs).Error)
	require.Zero(t, cntJobs)

	var stOther string
	require.NoError(t, db.Raw(`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'p3'`, rOther).Scan(&stOther).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage3), stOther)

	_, err = repo.InvalidateStage2And3SnapshotsForSlot(ctx, uuid.Nil)
	require.Error(t, err)
}

func TestRepository_InvalidateStage3SnapshotsForSlot_emptySlot(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()

	slot := uuid.MustParse("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee")
	n, err := repo.InvalidateStage3SnapshotsForSlot(ctx, slot)
	require.NoError(t, err)
	require.Zero(t, n)
}
