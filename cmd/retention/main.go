// Command retention runs job hard-delete retention (same logic as JobRetentionWorkflow).
//
// Usage:
//
//	retention run
//
// Requires JOBHOUND_DATABASE_URL (see specs/002-postgres-gorm-migrations/contracts/environment.md).
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	jobutils "github.com/andrewmysliuk/jobhound_core/internal/jobs/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/rs/zerolog"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch strings.ToLower(strings.TrimSpace(os.Args[1])) {
	case "run":
		appCfg := config.Load()
		log := logging.NewRoot(appCfg.Logging.Level, appCfg.Logging.Format, "retention")
		if err := run(log, appCfg); err != nil {
			log.Error().Err(err).Msg("retention run failed")
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: retention run\n\n")
	fmt.Fprintf(os.Stderr, "Hard-deletes jobs where created_at < now(UTC)−%d days (same cutoff as Temporal %s).\n",
		jobutils.Days, "JobRetentionWorkflow")
	fmt.Fprintf(os.Stderr, "Requires %s.\n", config.EnvDatabaseURL)
}

func run(log zerolog.Logger, appCfg config.Config) error {
	if strings.TrimSpace(appCfg.Database.URL) == "" {
		return fmt.Errorf("%s is required", config.EnvDatabaseURL)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	gdb, err := pgsql.Open(ctx, appCfg.Database)
	if err != nil {
		return err
	}
	defer func() {
		sqlDB, err := gdb.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}()

	repo := jobsstorage.NewRepository(pgsql.NewGetter(gdb))
	now := time.Now().UTC()
	cutoff := jobutils.CutoffUTC(now)
	n, err := repo.DeleteJobsCreatedBeforeUTC(ctx, cutoff)
	if err != nil {
		return err
	}
	log.Info().
		Int64("deleted_jobs", n).
		Str("cutoff_utc", cutoff.Format(time.RFC3339)).
		Msg("retention completed")
	return nil
}
