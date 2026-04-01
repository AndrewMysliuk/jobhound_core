//go:build integration

package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestMigrationsJobsSchema_integration applies SQL migrations and checks public.jobs
// matches contracts/jobs-schema.md. Requires a reachable Postgres (e.g. docker compose up -d).
//
// Build tag: go test -tags=integration ./...
// Env: JOBHOUND_MIGRATE_DATABASE_URL or JOBHOUND_DATABASE_URL (same precedence as cmd/migrate).
func TestMigrationsJobsSchema_integration(t *testing.T) {
	sqlDB := migrateUpAndOpenDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	cols, err := fetchJobsColumns(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	assertJobsSchema(t, cols)
}

// TestMigrationsIngestWatermarkAndPipelineRuns_integration checks 006 ingest_watermarks
// and pipeline_runs.broad_filter_key_hash after migrations (specs/006-cache-and-ingest).
func TestMigrationsIngestWatermarkAndPipelineRuns_integration(t *testing.T) {
	sqlDB := migrateUpAndOpenDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()
	var n int
	err := sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'ingest_watermarks'`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("ingest_watermarks column count: got %d want 3", n)
	}

	err = sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'pipeline_runs'
		  AND column_name = 'broad_filter_key_hash'`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pipeline_runs.broad_filter_key_hash: got count %d want 1", n)
	}
}

// TestMigrationsPipelineRunJobsCascadeOnJobDelete_integration verifies 007 §7: deleting a job row
// removes matching pipeline_run_jobs via FK ON DELETE CASCADE (006 retention alignment).
func TestMigrationsPipelineRunJobsCascadeOnJobDelete_integration(t *testing.T) {
	sqlDB := migrateUpAndOpenDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })

	jobID := "cascade_integration_job"
	_, err = tx.ExecContext(ctx, `
		INSERT INTO jobs (id, source, title, company, url, description, created_at, updated_at)
		VALUES ($1, 's', 't', 'c', 'https://example.com', 'd', NOW(), NOW())`,
		jobID)
	if err != nil {
		t.Fatal(err)
	}

	var runID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO pipeline_runs DEFAULT VALUES RETURNING id`).Scan(&runID); err != nil {
		t.Fatal(err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO pipeline_run_jobs (pipeline_run_id, job_id, status)
		VALUES ($1, $2, 'PASSED_STAGE_2')`,
		runID, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE id = $1`, jobID); err != nil {
		t.Fatal(err)
	}

	var cnt int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pipeline_run_jobs WHERE job_id = $1`, jobID).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("pipeline_run_jobs rows after job delete: got %d want 0 (CASCADE)", cnt)
	}

	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// TestMigrations007PipelineTables_integration asserts pipeline_runs / pipeline_run_jobs shape, index, and job_id FK CASCADE (007 contracts).
func TestMigrations007PipelineTables_integration(t *testing.T) {
	sqlDB := migrateUpAndOpenDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()

	runCols, err := fetchTableColumns(sqlDB, "pipeline_runs")
	if err != nil {
		t.Fatal(err)
	}
	assertTableColumns(t, runCols, map[string]struct {
		dataType string
		nullable string
	}{
		"id":                    {"bigint", "NO"},
		"created_at":            {"timestamp with time zone", "NO"},
		"broad_filter_key_hash": {"text", "YES"},
	})

	prjCols, err := fetchTableColumns(sqlDB, "pipeline_run_jobs")
	if err != nil {
		t.Fatal(err)
	}
	assertTableColumns(t, prjCols, map[string]struct {
		dataType string
		nullable string
	}{
		"pipeline_run_id": {"bigint", "NO"},
		"job_id":          {"text", "NO"},
		"status":          {"text", "NO"},
	})

	var nIdx int
	err = sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = 'pipeline_run_jobs'
		  AND indexname = 'pipeline_run_jobs_run_id_status_idx'`).Scan(&nIdx)
	if err != nil {
		t.Fatal(err)
	}
	if nIdx != 1 {
		t.Fatalf("pipeline_run_jobs_run_id_status_idx: got count %d want 1", nIdx)
	}

	var delRule string
	err = sqlDB.QueryRowContext(ctx, `
		SELECT rc.delete_rule
		FROM information_schema.referential_constraints rc
		JOIN information_schema.key_column_usage kcu
		  ON kcu.constraint_catalog = rc.constraint_catalog
		 AND kcu.constraint_schema = rc.constraint_schema
		 AND kcu.constraint_name = rc.constraint_name
		WHERE kcu.table_schema = 'public'
		  AND kcu.table_name = 'pipeline_run_jobs'
		  AND kcu.column_name = 'job_id'`).Scan(&delRule)
	if err != nil {
		t.Fatal(err)
	}
	if delRule != "CASCADE" {
		t.Fatalf("pipeline_run_jobs.job_id ON DELETE: got %q want CASCADE", delRule)
	}
}

func migrateUpAndOpenDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := dsnForIntegration()
	if dsn == "" {
		t.Skip("set JOBHOUND_DATABASE_URL or JOBHOUND_MIGRATE_DATABASE_URL (see docker-compose.yml example)")
	}

	ctx := context.Background()
	root := moduleRoot(t)
	migDir, err := filepath.Abs(filepath.Join(root, "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	src := "file://" + filepath.ToSlash(migDir)

	m, err := migrate.New(src, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = m.Close() })

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("first migrate up: %v", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("second migrate up (idempotent): %v", err)
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		t.Fatal(err)
	}
	return sqlDB
}

func dsnForIntegration() string {
	return config.LoadDatabaseFromEnv().MigrationDSN()
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from test working directory")
		}
		dir = parent
	}
}

type jobColumn struct {
	name     string
	dataType string
	nullable string
}

func fetchJobsColumns(sqlDB *sql.DB) ([]jobColumn, error) {
	return fetchTableColumns(sqlDB, "jobs")
}

func fetchTableColumns(sqlDB *sql.DB, table string) ([]jobColumn, error) {
	rows, err := sqlDB.Query(`
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []jobColumn
	for rows.Next() {
		var c jobColumn
		if err := rows.Scan(&c.name, &c.dataType, &c.nullable); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func assertTableColumns(t *testing.T, cols []jobColumn, want map[string]struct {
	dataType string
	nullable string
}) {
	t.Helper()
	if len(cols) != len(want) {
		names := make([]string, 0, len(cols))
		for _, c := range cols {
			names = append(names, c.name)
		}
		sort.Strings(names)
		t.Fatalf("column count: got %d want %d (%v)", len(cols), len(want), names)
	}
	byName := make(map[string]jobColumn, len(cols))
	for _, c := range cols {
		byName[c.name] = c
	}
	for name, exp := range want {
		c, ok := byName[name]
		if !ok {
			t.Errorf("missing column %q", name)
			continue
		}
		if c.dataType != exp.dataType {
			t.Errorf("column %q data_type: got %q want %q", name, c.dataType, exp.dataType)
		}
		if c.nullable != exp.nullable {
			t.Errorf("column %q is_nullable: got %q want %q", name, c.nullable, exp.nullable)
		}
	}
}

func assertJobsSchema(t *testing.T, cols []jobColumn) {
	t.Helper()
	assertTableColumns(t, cols, map[string]struct {
		dataType string
		nullable string
	}{
		"id":            {"text", "NO"},
		"source":        {"text", "NO"},
		"title":         {"text", "NO"},
		"company":       {"text", "NO"},
		"url":           {"text", "NO"},
		"apply_url":     {"text", "YES"},
		"description":   {"text", "NO"},
		"posted_at":     {"timestamp with time zone", "YES"},
		"user_id":       {"text", "YES"},
		"is_remote":     {"boolean", "YES"},
		"country_code":  {"text", "NO"},
		"salary_raw":    {"text", "NO"},
		"tags":          {"jsonb", "NO"},
		"position":      {"text", "YES"},
		"stage1_status": {"text", "YES"},
		"created_at":    {"timestamp with time zone", "NO"},
		"updated_at":    {"timestamp with time zone", "NO"},
	})
}
