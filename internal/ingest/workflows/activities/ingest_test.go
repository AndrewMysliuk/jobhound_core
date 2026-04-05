package ingest_activities

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubIncrCollector struct {
	name string
}

func (s stubIncrCollector) Name() string { return s.name }

func (stubIncrCollector) Fetch(context.Context) ([]domain.Job, error) {
	panic("Fetch should not run for incremental collector in this test")
}

func (stubIncrCollector) FetchIncremental(_ context.Context, cursor string) ([]domain.Job, string, error) {
	next := "v2"
	if cursor == "v2" {
		next = "v3"
	}
	// PostedAt inside default 7d window when Clock is fixed to 2026-04-02 (see test).
	j := domain.Job{
		ID: "j1", Source: "src", Title: "t", Company: "c", URL: "https://u",
		Description: "d", PostedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	return []domain.Job{j}, next, nil
}

type memJobs struct {
	saved int
}

func (m *memJobs) Save(context.Context, domain.Job) error { return nil }

func (m *memJobs) SaveIngest(context.Context, domain.Job) (bool, error) {
	m.saved++
	return false, nil
}

func (m *memJobs) GetByID(context.Context, string) (domain.Job, error) {
	return domain.Job{}, nil
}

func (m *memJobs) DeleteJobsCreatedBeforeUTC(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (m *memJobs) UpsertSlotJob(context.Context, uuid.UUID, string) error { return nil }

func (m *memJobs) ListSlotJobsPassedStage1(context.Context, uuid.UUID) ([]domain.Job, error) {
	return nil, nil
}

func (m *memJobs) ListPassedStage2JobsForRun(context.Context, int64) ([]domain.Job, error) {
	return nil, nil
}

func (m *memJobs) ListSlotStage1Jobs(context.Context, uuid.UUID, int, int) ([]jobs.JobListEntry, int64, error) {
	return nil, 0, nil
}

func (m *memJobs) ListPipelineRunStage2Jobs(context.Context, uuid.UUID, int64, jobs.ListBucket, int, int) ([]jobs.JobListEntry, int64, error) {
	return nil, 0, nil
}

func (m *memJobs) ListPipelineRunStage3Jobs(context.Context, uuid.UUID, int64, jobs.ListBucket, int, int) ([]jobs.JobListEntry, int64, error) {
	return nil, 0, nil
}

var _ jobs.JobRepository = (*memJobs)(nil)

func TestRunIngestSource_incrementalWatermark(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE ingest_watermarks (
			slot_id TEXT NOT NULL,
			source_id TEXT NOT NULL,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (slot_id, source_id)
		)
	`).Error)

	src := "testsrc"
	testSlot := uuid.MustParse("66666666-6666-4666-8666-666666666666")
	fixedNow := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	acts := &IngestActivities{
		Redis:      ingest.NewRedisCoordinator(rdb),
		Jobs:       &memJobs{},
		Watermarks: ingest.NewGormWatermarkStore(pgsql.NewGetter(db)),
		Collectors: map[string]collectors.Collector{
			ingest.NormalizeSourceID(src): stubIncrCollector{name: src},
		},
		BroadRules: pipeline.BroadFilterRules{},
		Clock:      func() time.Time { return fixedNow },
		Log:        logging.Nop(),
	}

	out, err := acts.RunIngestSource(ctx, ingestschema.IngestSourceInput{SlotID: testSlot, SourceID: src, ExplicitRefresh: false})
	require.NoError(t, err)
	require.True(t, out.UsedIncremental)
	require.True(t, out.WatermarkAdvanced)
	require.Equal(t, 1, out.JobsWritten)

	var cur string
	require.NoError(t, db.Raw(`SELECT cursor FROM ingest_watermarks WHERE slot_id = ? AND source_id = ?`, testSlot.String(), ingest.NormalizeSourceID(src)).Scan(&cur).Error)
	require.Equal(t, "v2", cur)

	out2, err := acts.RunIngestSource(ctx, ingestschema.IngestSourceInput{SlotID: testSlot, SourceID: src, ExplicitRefresh: true})
	require.NoError(t, err)
	require.NoError(t, db.Raw(`SELECT cursor FROM ingest_watermarks WHERE slot_id = ? AND source_id = ?`, testSlot.String(), ingest.NormalizeSourceID(src)).Scan(&cur).Error)
	require.Equal(t, "v3", cur)
	require.Equal(t, 1, out2.JobsWritten)
}

type stubMultiCollector struct {
	name string
}

func (s stubMultiCollector) Name() string { return s.name }

func (stubMultiCollector) Fetch(context.Context) ([]domain.Job, error) {
	panic("Fetch not used")
}

func (stubMultiCollector) FetchIncremental(_ context.Context, _ string) ([]domain.Job, string, error) {
	old := domain.Job{
		ID: "old", Source: "src", Title: "t", Company: "c", URL: "https://old",
		Description: "d", PostedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	fresh := domain.Job{
		ID: "fresh", Source: "src", Title: "t", Company: "c", URL: "https://fresh",
		Description: "d", PostedAt: time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC),
	}
	return []domain.Job{old, fresh}, "next", nil
}

func TestRunIngestSource_broadFilterSkipsNonPassing(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE ingest_watermarks (
			slot_id TEXT NOT NULL,
			source_id TEXT NOT NULL,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (slot_id, source_id)
		)
	`).Error)

	src := "multisrc"
	slot2 := uuid.MustParse("77777777-7777-4777-8777-777777777777")
	mj := &memJobs{}
	fixedNow := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	acts := &IngestActivities{
		Redis:      ingest.NewRedisCoordinator(rdb),
		Jobs:       mj,
		Watermarks: ingest.NewGormWatermarkStore(pgsql.NewGetter(db)),
		Collectors: map[string]collectors.Collector{
			ingest.NormalizeSourceID(src): stubMultiCollector{name: src},
		},
		BroadRules: pipeline.BroadFilterRules{},
		Clock:      func() time.Time { return fixedNow },
		Log:        logging.Nop(),
	}

	out, err := acts.RunIngestSource(ctx, ingestschema.IngestSourceInput{SlotID: slot2, SourceID: src, ExplicitRefresh: true})
	require.NoError(t, err)
	require.Equal(t, 1, out.JobsFilteredOut)
	require.Equal(t, 1, out.JobsWritten)
	require.Equal(t, 1, mj.saved)
}

func TestRunIngestSource_writesSlotJobMembership(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	memName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+memName+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
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
		`CREATE TABLE ingest_watermarks (
			slot_id TEXT NOT NULL,
			source_id TEXT NOT NULL,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (slot_id, source_id)
		)`,
	} {
		require.NoError(t, db.Exec(s).Error)
	}

	src := "slotmemsrc"
	testSlot := uuid.MustParse("88888888-8888-4888-8888-888888888888")
	fixedNow := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	jobsRepo := jobsstorage.NewRepository(pgsql.NewGetter(db))
	acts := &IngestActivities{
		Redis:      ingest.NewRedisCoordinator(rdb),
		Jobs:       jobsRepo,
		Watermarks: ingest.NewGormWatermarkStore(pgsql.NewGetter(db)),
		Collectors: map[string]collectors.Collector{
			ingest.NormalizeSourceID(src): stubMultiCollector{name: src},
		},
		BroadRules: pipeline.BroadFilterRules{},
		Clock:      func() time.Time { return fixedNow },
		Log:        logging.Nop(),
	}

	out, err := acts.RunIngestSource(ctx, ingestschema.IngestSourceInput{SlotID: testSlot, SourceID: src, ExplicitRefresh: true})
	require.NoError(t, err)
	require.Equal(t, 1, out.JobsWritten)

	var cnt int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM slot_jobs WHERE slot_id = ? AND job_id = 'fresh'`,
		testSlot.String(),
	).Scan(&cnt).Error)
	require.Equal(t, int64(1), cnt)

	var st *string
	require.NoError(t, db.Raw(`SELECT stage1_status FROM jobs WHERE id = 'fresh'`).Scan(&st).Error)
	require.NotNil(t, st)
	require.Equal(t, jobs.Stage1StatusPassed, *st)

	// Stage-2 input pool: slot_jobs ∩ jobs with PASSED_STAGE_1 (008 spec / contract §6).
	passed, err := jobsRepo.ListSlotJobsPassedStage1(ctx, testSlot)
	require.NoError(t, err)
	require.Len(t, passed, 1)
	require.Equal(t, "fresh", passed[0].ID)
}
