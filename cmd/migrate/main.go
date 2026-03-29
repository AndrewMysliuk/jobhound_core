package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) < 1 {
		return usageError()
	}

	dsn, err := config.MigrateDSNFromEnv()
	if err != nil {
		return err
	}

	migrationsDir := "migrations"
	if i := indexArg(args, "-path"); i >= 0 {
		if i+1 >= len(args) {
			return fmt.Errorf("-path requires a directory")
		}
		migrationsDir = args[i+1]
		args = append(args[:i], args[i+2:]...)
		if len(args) < 1 {
			return usageError()
		}
	}

	abs, err := filepath.Abs(migrationsDir)
	if err != nil {
		return err
	}
	src := "file://" + filepath.ToSlash(abs)

	m, err := migrate.New(src, dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	switch args[0] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		return nil
	case "down":
		n := 1
		if len(args) > 1 {
			var err error
			n, err = strconv.Atoi(args[1])
			if err != nil || n < 1 {
				return fmt.Errorf("down: optional argument must be a positive integer (steps), got %q", args[1])
			}
		}
		if err := m.Steps(-n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		return nil
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("version: nil (no migrations applied)")
				return nil
			}
			return err
		}
		fmt.Printf("version: %d dirty=%v\n", v, dirty)
		return nil
	case "force":
		if len(args) < 2 {
			return fmt.Errorf("force: requires version integer")
		}
		v, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("force: invalid version %q", args[1])
		}
		return m.Force(v)
	default:
		return usageError()
	}
}

func indexArg(args []string, name string) int {
	for i, a := range args {
		if a == name {
			return i
		}
	}
	return -1
}

func usageError() error {
	return fmt.Errorf(`usage: migrate [-path DIR] <command> [args]

commands:
  up              apply all pending migrations
  down [N]        apply N down migrations (default N=1)
  version         print current schema version
  force VERSION   set schema version (repair; use with care)

environment (see specs/002-postgres-gorm-migrations/contracts/environment.md):
  %s         — Postgres URL
  %s — optional override for migrate only`,
		config.EnvDatabaseURL, config.EnvMigrateDatabaseURL)
}
