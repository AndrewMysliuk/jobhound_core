package ingest_activities

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubIncrCollector struct {
	name string
}

func (s stubIncrCollector) Name() string { return s.name }

func (stubIncrCollector) Fetch(context.Context) ([]schema.Job, error) {
	panic("Fetch should not run for incremental collector in this test")
}

func (stubIncrCollector) FetchIncremental(_ context.Context, cursor string) ([]schema.Job, string, error) {
	next := "v2"
	if cursor == "v2" {
		next = "v3"
	}
	j := schema.Job{
		ID: "j1", Source: "src", Title: "t", Company: "c", URL: "https://u",
		Description: "d", PostedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	return []schema.Job{j}, next, nil
}

type memJobs struct {
	saved int
}

func (m *memJobs) Save(context.Context, schema.Job) error { return nil }

func (m *memJobs) SaveIngest(context.Context, schema.Job) (bool, error) {
	m.saved++
	return false, nil
}

func (m *memJobs) GetByID(context.Context, string) (schema.Job, error) {
	return schema.Job{}, nil
}

func (m *memJobs) ListFirstSeenSince(context.Context, time.Time) ([]schema.Job, error) {
	return nil, nil
}

func (m *memJobs) DeleteJobsLastSeenBeforeUTC(context.Context, time.Time) (int64, error) {
	return 0, nil
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
			source_id TEXT NOT NULL,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (source_id)
		)
	`).Error)

	src := "testsrc"
	acts := &IngestActivities{
		Redis:      ingest.NewRedisCoordinator(rdb),
		Jobs:       &memJobs{},
		Watermarks: ingest.NewGormWatermarkStore(pgsql.NewGetter(db)),
		Collectors: map[string]collectors.Collector{
			ingest.NormalizeSourceID(src): stubIncrCollector{name: src},
		},
		Log: logging.Nop(),
	}

	out, err := acts.RunIngestSource(ctx, ingestschema.IngestSourceInput{SourceID: src, ExplicitRefresh: false})
	require.NoError(t, err)
	require.True(t, out.UsedIncremental)
	require.True(t, out.WatermarkAdvanced)
	require.Equal(t, 1, out.JobsWritten)

	var cur string
	require.NoError(t, db.Raw(`SELECT cursor FROM ingest_watermarks WHERE source_id = ?`, ingest.NormalizeSourceID(src)).Scan(&cur).Error)
	require.Equal(t, "v2", cur)

	out2, err := acts.RunIngestSource(ctx, ingestschema.IngestSourceInput{SourceID: src, ExplicitRefresh: true})
	require.NoError(t, err)
	require.NoError(t, db.Raw(`SELECT cursor FROM ingest_watermarks WHERE source_id = ?`, ingest.NormalizeSourceID(src)).Scan(&cur).Error)
	require.Equal(t, "v3", cur)
	require.Equal(t, 1, out2.JobsWritten)
}

type stubMultiCollector struct {
	name string
}

func (s stubMultiCollector) Name() string { return s.name }

func (stubMultiCollector) Fetch(context.Context) ([]schema.Job, error) {
	panic("Fetch not used")
}

func (stubMultiCollector) FetchIncremental(_ context.Context, _ string) ([]schema.Job, string, error) {
	old := schema.Job{
		ID: "old", Source: "src", Title: "t", Company: "c", URL: "https://old",
		Description: "d", PostedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	fresh := schema.Job{
		ID: "fresh", Source: "src", Title: "t", Company: "c", URL: "https://fresh",
		Description: "d", PostedAt: time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC),
	}
	return []schema.Job{old, fresh}, "next", nil
}

func TestRunIngestSource_savesEveryFetchedJob(t *testing.T) {
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
			source_id TEXT NOT NULL,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (source_id)
		)
	`).Error)

	src := "multisrc"
	mj := &memJobs{}
	acts := &IngestActivities{
		Redis:      ingest.NewRedisCoordinator(rdb),
		Jobs:       mj,
		Watermarks: ingest.NewGormWatermarkStore(pgsql.NewGetter(db)),
		Collectors: map[string]collectors.Collector{
			ingest.NormalizeSourceID(src): stubMultiCollector{name: src},
		},
		Log: logging.Nop(),
	}

	out, err := acts.RunIngestSource(ctx, ingestschema.IngestSourceInput{SourceID: src, ExplicitRefresh: true})
	require.NoError(t, err)
	require.Equal(t, 2, out.JobsWritten)
	require.Equal(t, 2, mj.saved)
}

func TestRunIngestSource_persistsJob(t *testing.T) {
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
			sources TEXT NOT NULL DEFAULT '[]',
			title TEXT NOT NULL DEFAULT '',
			company TEXT NOT NULL DEFAULT '',
			company_key TEXT NOT NULL DEFAULT '',
			company_website TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			apply_url TEXT,
			description TEXT NOT NULL DEFAULT '',
			posted_at TIMESTAMP,
			location TEXT NOT NULL DEFAULT '{}',
			salary_raw TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			position TEXT,
			first_seen_at TIMESTAMP NOT NULL,
			last_seen_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE ingest_watermarks (
			source_id TEXT NOT NULL,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (source_id)
		)`,
	} {
		require.NoError(t, db.Exec(s).Error)
	}

	src := "memsrc"
	jobsRepo := jobsstorage.NewRepository(pgsql.NewGetter(db))
	acts := &IngestActivities{
		Redis:      ingest.NewRedisCoordinator(rdb),
		Jobs:       jobsRepo,
		Watermarks: ingest.NewGormWatermarkStore(pgsql.NewGetter(db)),
		Collectors: map[string]collectors.Collector{
			ingest.NormalizeSourceID(src): stubMultiCollector{name: src},
		},
		Log: logging.Nop(),
	}

	out, err := acts.RunIngestSource(ctx, ingestschema.IngestSourceInput{SourceID: src, ExplicitRefresh: true})
	require.NoError(t, err)
	require.Equal(t, 2, out.JobsWritten)

	var sources string
	require.NoError(t, db.Raw(`SELECT sources FROM jobs WHERE id = 'fresh'`).Scan(&sources).Error)
	require.Contains(t, sources, "src")
	require.NoError(t, db.Raw(`SELECT sources FROM jobs WHERE id = 'old'`).Scan(&sources).Error)
	require.Contains(t, sources, "src")
	var first, last time.Time
	require.NoError(t, db.Raw(`SELECT first_seen_at, last_seen_at FROM jobs WHERE id = 'fresh'`).Row().Scan(&first, &last))
	require.False(t, first.IsZero())
	require.False(t, last.IsZero())
}
