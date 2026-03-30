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
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	cols, err := fetchJobsColumns(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	assertJobsSchema(t, cols)
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
	rows, err := sqlDB.Query(`
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'jobs'
		ORDER BY ordinal_position`)
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

func assertJobsSchema(t *testing.T, cols []jobColumn) {
	t.Helper()

	want := map[string]struct {
		dataType string
		nullable string
	}{
		"id":           {"text", "NO"},
		"source":       {"text", "NO"},
		"title":        {"text", "NO"},
		"company":      {"text", "NO"},
		"url":          {"text", "NO"},
		"apply_url":    {"text", "YES"},
		"description":  {"text", "NO"},
		"posted_at":    {"timestamp with time zone", "YES"},
		"user_id":      {"text", "YES"},
		"is_remote":    {"boolean", "YES"},
		"country_code": {"text", "NO"},
		"created_at":   {"timestamp with time zone", "NO"},
		"updated_at":   {"timestamp with time zone", "NO"},
	}

	if len(cols) != len(want) {
		names := make([]string, 0, len(cols))
		for _, c := range cols {
			names = append(names, c.name)
		}
		sort.Strings(names)
		t.Fatalf("jobs column count: got %d want %d (%v)", len(cols), len(want), names)
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
