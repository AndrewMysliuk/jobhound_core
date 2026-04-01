package ingest_activities

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
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
			source_id TEXT PRIMARY KEY,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL
		)
	`).Error)

	src := "testsrc"
	fixedNow := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	acts := &IngestActivities{
		Redis:      ingest.NewRedisCoordinator(rdb),
		Jobs:       &memJobs{},
		Watermarks: ingest.NewGormWatermarkStore(pgsql.NewGetter(db)),
		Collectors: map[string]pipeline.Collector{
			ingest.NormalizeSourceID(src): stubIncrCollector{name: src},
		},
		BroadRules: pipeline.BroadFilterRules{},
		Clock:      func() time.Time { return fixedNow },
	}

	out, err := acts.RunIngestSource(ctx, IngestSourceInput{SourceID: src, ExplicitRefresh: false})
	require.NoError(t, err)
	require.True(t, out.UsedIncremental)
	require.True(t, out.WatermarkAdvanced)
	require.Equal(t, 1, out.JobsWritten)

	var cur string
	require.NoError(t, db.Raw(`SELECT cursor FROM ingest_watermarks WHERE source_id = ?`, ingest.NormalizeSourceID(src)).Scan(&cur).Error)
	require.Equal(t, "v2", cur)

	out2, err := acts.RunIngestSource(ctx, IngestSourceInput{SourceID: src, ExplicitRefresh: true})
	require.NoError(t, err)
	require.NoError(t, db.Raw(`SELECT cursor FROM ingest_watermarks WHERE source_id = ?`, ingest.NormalizeSourceID(src)).Scan(&cur).Error)
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
			source_id TEXT PRIMARY KEY,
			cursor TEXT,
			updated_at TIMESTAMP NOT NULL
		)
	`).Error)

	src := "multisrc"
	mj := &memJobs{}
	fixedNow := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	acts := &IngestActivities{
		Redis:      ingest.NewRedisCoordinator(rdb),
		Jobs:       mj,
		Watermarks: ingest.NewGormWatermarkStore(pgsql.NewGetter(db)),
		Collectors: map[string]pipeline.Collector{
			ingest.NormalizeSourceID(src): stubMultiCollector{name: src},
		},
		BroadRules: pipeline.BroadFilterRules{},
		Clock:      func() time.Time { return fixedNow },
	}

	out, err := acts.RunIngestSource(ctx, IngestSourceInput{SourceID: src, ExplicitRefresh: true})
	require.NoError(t, err)
	require.Equal(t, 1, out.JobsFilteredOut)
	require.Equal(t, 1, out.JobsWritten)
	require.Equal(t, 1, mj.saved)
}
