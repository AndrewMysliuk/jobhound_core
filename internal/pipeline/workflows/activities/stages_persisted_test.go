package pipeline_activities

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
	pipelinestorage "github.com/andrewmysliuk/jobhound_core/internal/pipeline/storage"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testSQLite(t *testing.T) *gorm.DB {
	t.Helper()
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
	} {
		require.NoError(t, db.Exec(s).Error)
	}
	return db
}

func seedJobRow(t *testing.T, db *gorm.DB, id, description string) {
	t.Helper()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Exec(`
		INSERT INTO jobs (id, source, title, company, url, description, tags, posted_at, is_remote, country_code, created_at, updated_at)
		VALUES (?, 'src', 't', 'c', 'https://x', ?, '[]', ?, 1, 'DE', ?, ?)`,
		id, description, now, now, now).Error)
}

func TestRunPersistedPipelineStages_persistsStage2AndScoresCappedBatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := testSQLite(t)
	getter := pgsql.NewGetter(db)
	runRepo := pipelinestorage.NewRepository(getter)
	jobRepo := storage.NewRepository(getter)

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	ids := []string{"j1", "j2", "j3", "j4", "j5", "j6", "j7"}
	for _, id := range ids {
		seedJobRow(t, db, id, "backend golang remote")
	}

	slotID := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	runID, err := runRepo.CreateRun(ctx, &slotID)
	require.NoError(t, err)

	broadHash, err := ingest.BroadFilterKeyHashFromRules(pipeline.BroadFilterRules{
		RoleSynonyms:     []string{"go"},
		RemoteOnly:       true,
		CountryAllowlist: []string{"de"},
	}, []string{"djinni"}, slotID, nil)
	require.NoError(t, err)

	scorer := stubScorer(func(_ context.Context, _ string, j domain.Job) (domain.ScoredJob, error) {
		return domain.ScoredJob{Job: j, Score: 80, Reason: "ok"}, nil
	})

	a := &Activities{
		Clock:  func() time.Time { return now },
		Scorer: scorer,
		Runs:   runRepo,
		Jobs:   jobRepo,
	}

	jobs := make([]domain.Job, len(ids))
	for i, id := range ids {
		jobs[i] = domain.Job{
			ID: id, Title: "Go Dev", Description: "backend golang",
			PostedAt: now.Add(-24 * time.Hour), Remote: ptr(true), CountryCode: "DE",
		}
	}

	out, err := a.RunPersistedPipelineStages(ctx, pipelineschema.PersistedPipelineStagesInput{
		PipelineRunID:      runID,
		BroadFilterKeyHash: broadHash,
		Jobs:               jobs,
		BroadRules: pipeline.BroadFilterRules{
			RoleSynonyms:     []string{"go"},
			RemoteOnly:       true,
			CountryAllowlist: []string{"de"},
		},
		KeywordRules: pipeline.KeywordRules{Include: []string{"backend"}},
		Profile:      "cv",
	})
	require.NoError(t, err)
	require.Len(t, out.AfterBroad, 7)
	require.Len(t, out.AfterKeywords, 7)
	require.Len(t, out.Scored, pipeutils.MaxStage3JobsPerPipelineRunExecution, "cap N per execution")

	var nPassed2 int64
	require.NoError(t, db.Model(&pipelinestorage.PipelineRunJob{}).
		Where("pipeline_run_id = ? AND status = ?", runID, string(pipeline.RunJobPassedStage2)).
		Count(&nPassed2).Error)
	require.Equal(t, int64(2), nPassed2, "two jobs remain backlog PASSED_STAGE_2")

	var nTerm int64
	require.NoError(t, db.Model(&pipelinestorage.PipelineRunJob{}).
		Where("pipeline_run_id = ? AND status IN ?", runID, []string{string(pipeline.RunJobPassedStage3), string(pipeline.RunJobRejectedStage3)}).
		Count(&nTerm).Error)
	require.Equal(t, int64(5), nTerm)

	var gotHash *string
	require.NoError(t, db.Raw(`SELECT broad_filter_key_hash FROM pipeline_runs WHERE id = ?`, runID).Scan(&gotHash).Error)
	require.NotNil(t, gotHash)
	require.Equal(t, broadHash, *gotHash)
}

func TestRunPersistedPipelineStages_stage3RejectScore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := testSQLite(t)
	getter := pgsql.NewGetter(db)
	runRepo := pipelinestorage.NewRepository(getter)
	jobRepo := storage.NewRepository(getter)

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	seedJobRow(t, db, "a", "backend")

	runID, err := runRepo.CreateRun(ctx, nil)
	require.NoError(t, err)

	scorer := stubScorer(func(_ context.Context, _ string, j domain.Job) (domain.ScoredJob, error) {
		return domain.ScoredJob{Job: j, Score: 40, Reason: "low"}, nil
	})

	a := &Activities{
		Clock:  func() time.Time { return now },
		Scorer: scorer,
		Runs:   runRepo,
		Jobs:   jobRepo,
	}

	_, err = a.RunPersistedPipelineStages(ctx, pipelineschema.PersistedPipelineStagesInput{
		PipelineRunID: runID,
		Jobs: []domain.Job{{
			ID: "a", Title: "Go", Description: "backend",
			PostedAt: now.Add(-24 * time.Hour), Remote: ptr(true), CountryCode: "DE",
		}},
		BroadRules: pipeline.BroadFilterRules{
			RoleSynonyms:     []string{"go"},
			RemoteOnly:       true,
			CountryAllowlist: []string{"de"},
		},
		KeywordRules: pipeline.KeywordRules{Include: []string{"backend"}},
		Profile:      "cv",
	})
	require.NoError(t, err)

	var st string
	require.NoError(t, db.Raw(
		`SELECT status FROM pipeline_run_jobs WHERE pipeline_run_id = ? AND job_id = ?`,
		runID, "a").Scan(&st).Error)
	require.Equal(t, string(pipeline.RunJobRejectedStage3), st)
}

type stubScorer func(context.Context, string, domain.Job) (domain.ScoredJob, error)

func (f stubScorer) Score(ctx context.Context, profile string, job domain.Job) (domain.ScoredJob, error) {
	return f(ctx, profile, job)
}
