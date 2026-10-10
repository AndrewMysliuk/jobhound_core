//go:build integration

package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestMigrationsJobsSchema_integration applies SQL migrations and checks public.jobs
// matches the v2 GORM model. Requires a reachable Postgres (e.g. docker compose up -d).
//
// Build tag: go test -tags=integration ./internal/platform/pgsql
// Env: JOBHOUND_MIGRATE_DATABASE_URL or JOBHOUND_DATABASE_URL (same precedence as cmd/migrate).
func TestMigrationsJobsSchema_integration(t *testing.T) {
	sqlDB := migrateUpAndOpenDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	cols, err := fetchTableColumns(sqlDB, "jobs")
	if err != nil {
		t.Fatal(err)
	}
	assertTableColumns(t, cols, map[string]struct {
		dataType string
		nullable string
	}{
		"id":              {"text", "NO"},
		"sources":         {"ARRAY", "NO"},
		"title":           {"text", "NO"},
		"company":         {"text", "NO"},
		"company_key":     {"text", "NO"},
		"company_website": {"text", "NO"},
		"url":             {"text", "NO"},
		"apply_url":       {"text", "YES"},
		"description":     {"text", "NO"},
		"posted_at":       {"timestamp with time zone", "YES"},
		"location":        {"jsonb", "NO"},
		"salary_raw":      {"text", "NO"},
		"tags":            {"jsonb", "NO"},
		"position":        {"text", "YES"},
		"first_seen_at":   {"timestamp with time zone", "NO"},
		"last_seen_at":    {"timestamp with time zone", "NO"},
		"created_at":      {"timestamp with time zone", "NO"},
		"updated_at":      {"timestamp with time zone", "NO"},
	})

	ctx := context.Background()
	for _, name := range []string{"jobs_first_seen_at_idx", "jobs_last_seen_at_idx", "jobs_company_key_idx"} {
		var n int
		err := sqlDB.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = 'jobs' AND indexname = $1`, name).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("index %s: got count %d want 1", name, n)
		}
	}
}

// TestMigrationsProfileRunsAndMatches_integration checks profile_runs, profile_matches,
// the bucket check, and that an insert can omit user_status.
func TestMigrationsProfileRunsAndMatches_integration(t *testing.T) {
	sqlDB := migrateUpAndOpenDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	runCols, err := fetchTableColumns(sqlDB, "profile_runs")
	if err != nil {
		t.Fatal(err)
	}
	assertTableColumns(t, runCols, map[string]struct {
		dataType string
		nullable string
	}{
		"id":              {"bigint", "NO"},
		"profile_id":      {"text", "NO"},
		"status":          {"text", "NO"},
		"started_at":      {"timestamp with time zone", "NO"},
		"finished_at":     {"timestamp with time zone", "YES"},
		"jobs_scored":     {"integer", "NO"},
		"sources_skipped": {"integer", "NO"},
		"idempotency_key": {"uuid", "NO"},
	})

	ctx := context.Background()
	var uniqueCount int
	err = sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = 'profile_runs'
		  AND indexdef LIKE '%UNIQUE%' AND indexdef LIKE '%idempotency_key%'`).Scan(&uniqueCount)
	if err != nil {
		t.Fatal(err)
	}
	if uniqueCount != 1 {
		t.Fatalf("idempotency_key unique index: got %d want 1", uniqueCount)
	}

	matchCols, err := fetchTableColumns(sqlDB, "profile_matches")
	if err != nil {
		t.Fatal(err)
	}
	assertTableColumns(t, matchCols, map[string]struct {
		dataType string
		nullable string
	}{
		"profile_id":  {"text", "NO"},
		"job_id":      {"text", "NO"},
		"bucket":      {"text", "NO"},
		"score":       {"integer", "NO"},
		"signals":     {"jsonb", "NO"},
		"user_status": {"text", "NO"},
		"run_id":      {"bigint", "NO"},
		"updated_at":  {"timestamp with time zone", "NO"},
	})

	var userDefault string
	err = sqlDB.QueryRowContext(ctx, `
		SELECT column_default FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'profile_matches' AND column_name = 'user_status'`,
	).Scan(&userDefault)
	if err != nil {
		t.Fatal(err)
	}
	if userDefault != "'NEW'::text" {
		t.Fatalf("user_status default: got %q want 'NEW'::text", userDefault)
	}

	var indexdef string
	err = sqlDB.QueryRowContext(ctx, `
		SELECT indexdef FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = 'profile_matches' AND indexname = 'profile_matches_list'`,
	).Scan(&indexdef)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(indexdef, "(profile_id, bucket, user_status, score DESC)") {
		t.Fatalf("profile_matches_list: got %q", indexdef)
	}

	checks := map[string][2]string{
		"profile_matches_bucket_check":      {"PASSED", "REJECTED"},
		"profile_matches_user_status_check": {"NEW", "HIDDEN"},
	}
	for name, pair := range checks {
		var def string
		err = sqlDB.QueryRowContext(ctx, `
			SELECT pg_get_constraintdef(oid) FROM pg_constraint
			WHERE conname = $1`, name).Scan(&def)
		if err != nil {
			t.Fatalf("constraint %s: %v", name, err)
		}
		if !strings.Contains(def, pair[0]) || !strings.Contains(def, pair[1]) {
			t.Fatalf("constraint %s: got %q", name, def)
		}
	}

	pkCols, err := primaryKeyColumns(sqlDB, "profile_matches")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pkCols, ",") != "profile_id,job_id" {
		t.Fatalf("profile_matches primary key: got %v", pkCols)
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
		  AND kcu.table_name = 'profile_matches'
		  AND kcu.column_name = 'job_id'`).Scan(&delRule)
	if err != nil {
		t.Fatal(err)
	}
	if delRule != "CASCADE" {
		t.Fatalf("profile_matches.job_id ON DELETE: got %q want CASCADE", delRule)
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO jobs (id, title, first_seen_at, last_seen_at)
		VALUES ('m1-job', 'Engineer', NOW(), NOW())`); err != nil {
		t.Fatal(err)
	}
	var runID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO profile_runs (profile_id, status, started_at, idempotency_key)
		VALUES ('andrew', 'RUNNING', NOW(), '11111111-1111-1111-1111-111111111111')
		RETURNING id`).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO profile_matches (profile_id, job_id, bucket, score, signals, run_id, updated_at)
		VALUES ('andrew', 'm1-job', 'PASSED', 0, '[]'::jsonb, $1, NOW())`, runID); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT user_status FROM profile_matches WHERE profile_id = 'andrew' AND job_id = 'm1-job'`,
	).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "NEW" {
		t.Fatalf("omitted user_status: got %q want NEW", status)
	}
}

// TestMigrationsIngestWatermarks_integration checks ingest_watermarks has no slot_id.
func TestMigrationsIngestWatermarks_integration(t *testing.T) {
	sqlDB := migrateUpAndOpenDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	cols, err := fetchTableColumns(sqlDB, "ingest_watermarks")
	if err != nil {
		t.Fatal(err)
	}
	assertTableColumns(t, cols, map[string]struct {
		dataType string
		nullable string
	}{
		"source_id":  {"text", "NO"},
		"cursor":     {"text", "YES"},
		"updated_at": {"timestamp with time zone", "NO"},
	})

	pkCols, err := primaryKeyColumns(sqlDB, "ingest_watermarks")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pkCols, ",") != "source_id" {
		t.Fatalf("ingest_watermarks primary key: got %v", pkCols)
	}
}

func primaryKeyColumns(sqlDB *sql.DB, table string) ([]string, error) {
	rows, err := sqlDB.Query(`
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY (i.indkey)
		WHERE i.indrelid = $1::regclass AND i.indisprimary
		ORDER BY array_position(i.indkey, a.attnum)`, "public."+table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

// TestMigrationsDroppedTables_integration checks v1 slot and pipeline tables are gone.
func TestMigrationsDroppedTables_integration(t *testing.T) {
	sqlDB := migrateUpAndOpenDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()
	var n int
	err := sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name IN (
			'slots', 'slot_jobs', 'slot_idempotency_keys', 'user_profile', 'pipeline_runs', 'pipeline_run_jobs'
		)`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("dropped tables still present: count %d", n)
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
