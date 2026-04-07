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
			stage2_status TEXT NOT NULL,
			stage3_status TEXT,
			stage3_rationale TEXT,
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
	require.NoError(t, repo.SetRunJobStage3Rationale(ctx, runID, "job-a", "  good fit  "))

	var got PipelineRunJob
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = ?", runID, "job-a").First(&got).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage2), got.Stage2Status)
	require.NotNil(t, got.Stage3Status)
	require.Equal(t, string(pipeline.RunJobPassedStage3), *got.Stage3Status)
	var rat *string
	require.NoError(t, db.Raw(
		`SELECT stage3_rationale FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = ?`,
		runID, "job-a").Scan(&rat).Error)
	require.NotNil(t, rat)
	require.Equal(t, "good fit", *rat)
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

// Eligible pool: stage2 passed and stage3 not yet set; terminal stage-3 rows are excluded from selection.
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
	var gotJ PipelineRunJob
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = ?", runID, "j").First(&gotJ).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage2), gotJ.Stage2Status)
	require.NotNil(t, gotJ.Stage3Status)
	require.Equal(t, string(pipeline.RunJobPassedStage3), *gotJ.Stage3Status)
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

	require.NoError(t, repo.SetRunJobStage3Rationale(ctx, runA, "j1", "inv-should-clear"))
	require.NoError(t, repo.SetRunJobStage3Rationale(ctx, runA, "j2", "also-clear"))

	n, err := repo.InvalidateStage3SnapshotsForSlot(ctx, slotA)
	require.NoError(t, err)
	require.Equal(t, int64(2), n)

	var rj1, rj2, rj3, rj4 PipelineRunJob
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = 'j1'", runA).First(&rj1).Error)
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = 'j2'", runA).First(&rj2).Error)
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = 'j3'", runA).First(&rj3).Error)
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = 'j4'", runA).First(&rj4).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage2), rj1.Stage2Status)
	require.Nil(t, rj1.Stage3Status)
	require.Equal(t, string(pipeline.RunJobPassedStage2), rj2.Stage2Status)
	require.Nil(t, rj2.Stage3Status)
	require.Equal(t, string(pipeline.RunJobPassedStage2), rj3.Stage2Status)
	require.Nil(t, rj3.Stage3Status)
	require.Equal(t, string(pipeline.RunJobRejectedStage2), rj4.Stage2Status)

	var ratJ1, ratJ2 *string
	require.NoError(t, db.Raw(`SELECT stage3_rationale FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'j1'`, runA).Scan(&ratJ1).Error)
	require.NoError(t, db.Raw(`SELECT stage3_rationale FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'j2'`, runA).Scan(&ratJ2).Error)
	require.Nil(t, ratJ1)
	require.Nil(t, ratJ2)

	var rj5 PipelineRunJob
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = 'j5'", runB).First(&rj5).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage2), rj5.Stage2Status)
	require.NotNil(t, rj5.Stage3Status)
	require.Equal(t, string(pipeline.RunJobPassedStage3), *rj5.Stage3Status)

	var rj6 PipelineRunJob
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = 'j6'", runLegacy).First(&rj6).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage2), rj6.Stage2Status)
	require.NotNil(t, rj6.Stage3Status)
	require.Equal(t, string(pipeline.RunJobRejectedStage3), *rj6.Stage3Status)

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

	var rp3 PipelineRunJob
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = 'p3'", rOther).First(&rp3).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage2), rp3.Stage2Status)
	require.NotNil(t, rp3.Stage3Status)
	require.Equal(t, string(pipeline.RunJobPassedStage3), *rp3.Stage3Status)

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

func TestRepository_ManualPatchStage2Bucket(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()
	runID, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	seedJob(t, db, "mj")
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "mj", pipeline.RunJobPassedStage2))

	require.NoError(t, repo.ManualPatchStage2Bucket(ctx, runID, "mj", false))
	var st2 string
	require.NoError(t, db.Raw(`SELECT stage2_status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'mj'`, runID).Scan(&st2).Error)
	require.Equal(t, string(pipeline.RunJobRejectedStage2), st2)

	require.NoError(t, repo.ManualPatchStage2Bucket(ctx, runID, "mj", true))
	require.NoError(t, db.Raw(`SELECT stage2_status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'mj'`, runID).Scan(&st2).Error)
	require.Equal(t, string(pipeline.RunJobPassedStage2), st2)

	err = repo.ManualPatchStage2Bucket(ctx, runID, "missing", true)
	require.ErrorIs(t, err, pipeline.ErrManualPatchNotInScope)
}

func TestRepository_ManualPatchStage2Bucket_clearsStage3(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()
	runID, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	seedJob(t, db, "mx")
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "mx", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "mx", pipeline.RunJobPassedStage3))
	require.NoError(t, repo.SetRunJobStage3Rationale(ctx, runID, "mx", "x"))

	require.NoError(t, repo.ManualPatchStage2Bucket(ctx, runID, "mx", false))
	var row PipelineRunJob
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = 'mx'", runID).First(&row).Error)
	require.Equal(t, string(pipeline.RunJobRejectedStage2), row.Stage2Status)
	require.Nil(t, row.Stage3Status)
	require.Nil(t, row.Stage3Rationale)
}

func TestRepository_ManualPatchStage3Bucket(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(pgsql.NewGetter(db))
	ctx := context.Background()
	runID, err := repo.CreateRun(ctx, nil)
	require.NoError(t, err)
	seedJob(t, db, "m3")
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "m3", pipeline.RunJobPassedStage2))
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "m3", pipeline.RunJobRejectedStage3))
	require.NoError(t, repo.SetRunJobStage3Rationale(ctx, runID, "m3", "llm said no"))

	require.NoError(t, repo.ManualPatchStage3Bucket(ctx, runID, "m3", true))
	var rm3 PipelineRunJob
	require.NoError(t, db.Where("pipeline_run_id = ? AND job_id = 'm3'", runID).First(&rm3).Error)
	require.NotNil(t, rm3.Stage3Status)
	require.Equal(t, string(pipeline.RunJobPassedStage3), *rm3.Stage3Status)
	var cleared *string
	require.NoError(t, db.Raw(`SELECT stage3_rationale FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = 'm3'`, runID).Scan(&cleared).Error)
	require.Nil(t, cleared)

	seedJob(t, db, "m2only")
	require.NoError(t, repo.SetRunJobStatus(ctx, runID, "m2only", pipeline.RunJobPassedStage2))
	err = repo.ManualPatchStage3Bucket(ctx, runID, "m2only", true)
	require.ErrorIs(t, err, pipeline.ErrManualPatchNotInScope)
}
