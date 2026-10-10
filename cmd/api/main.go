// Command api runs the browser-facing JSON HTTP API (composition only).
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/weworkremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	profilesimpl "github.com/andrewmysliuk/jobhound_core/internal/profiles/impl"
	publicapihandlers "github.com/andrewmysliuk/jobhound_core/internal/publicapi/handlers"
	scoringimpl "github.com/andrewmysliuk/jobhound_core/internal/scoring/impl"
	scoringstorage "github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
	scoringworkflows "github.com/andrewmysliuk/jobhound_core/internal/scoring/workflows"
	zlog "github.com/rs/zerolog/log"
	"go.temporal.io/sdk/client"
)

const shutdownTimeout = 30 * time.Second

// knownCollectorSourceIDs is the allow-list for profile YAML sources.
var knownCollectorSourceIDs = []string{
	europeremotely.SourceName,
	workingnomads.SourceName,
	himalayas.SourceName,
	remotifyeurope.SourceName,
	weworkremotely.SourceName,
	wellfound.SourceName,
	vuejobs.SourceName,
	golangcafe.SourceName,
	builtin.SourceName,
}

func main() {
	appCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	log := logging.NewRoot(appCfg.Logging.Level, appCfg.Logging.Format, "api")
	zlog.Logger = log

	if strings.TrimSpace(appCfg.Database.URL) == "" {
		log.Error().Str("env", config.EnvDatabaseURL).Msg("database url is required")
		os.Exit(1)
	}
	temporalCfg, err := config.LoadTemporalFromEnv()
	if err != nil {
		log.Error().Err(err).Msg("temporal config")
		os.Exit(1)
	}

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 15*time.Second)
	gdb, err := pgsql.Open(dbCtx, appCfg.Database)
	dbCancel()
	if err != nil {
		log.Error().Err(err).Msg("database open")
		os.Exit(1)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		log.Error().Err(err).Msg("database pool")
		os.Exit(1)
	}
	defer func() { _ = sqlDB.Close() }()

	temporalClient, err := client.Dial(client.Options{
		HostPort:  temporalCfg.Address,
		Namespace: temporalCfg.Namespace,
	})
	if err != nil {
		log.Error().Err(err).Msg("temporal dial")
		os.Exit(1)
	}
	defer temporalClient.Close()

	getter := pgsql.NewGetter(gdb)
	profileStore := profilesimpl.NewFileStore(appCfg.ProfilesDir, knownCollectorSourceIDs)
	scoringSvc := scoringimpl.New(profileStore, scoringstorage.NewRepository(getter), jobsstorage.NewRepository(getter), &scoringworkflows.Starter{
		Client:    temporalClient,
		TaskQueue: temporalCfg.TaskQueue,
	})

	handler := publicapihandlers.NewHTTPHandler(appCfg.API.CORSAllowedOrigins, publicapihandlers.Deps{
		Logger:   log,
		Profiles: profileStore,
		Scoring:  scoringSvc,
	})

	srv := &http.Server{
		Addr:    appCfg.API.Listen,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("listen", appCfg.API.Listen).Msg("listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-errCh:
		log.Error().Err(err).Msg("http server")
		os.Exit(1)
	case <-quit:
		shutdownCtx, scancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer scancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("shutdown")
			os.Exit(1)
		}
		log.Info().Msg("stopped")
	}
}
