// Command api runs the browser-facing JSON HTTP API (composition only).
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	pipelinestorage "github.com/andrewmysliuk/jobhound_core/internal/pipeline/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	profileimpl "github.com/andrewmysliuk/jobhound_core/internal/profile/impl"
	profilestorage "github.com/andrewmysliuk/jobhound_core/internal/profile/storage"
	publicapihandlers "github.com/andrewmysliuk/jobhound_core/internal/publicapi/handlers"
	slotsimpl "github.com/andrewmysliuk/jobhound_core/internal/slots/impl"
	slotstorage "github.com/andrewmysliuk/jobhound_core/internal/slots/storage"
	slotsutils "github.com/andrewmysliuk/jobhound_core/internal/slots/utils"
	"go.temporal.io/sdk/client"
)

const shutdownTimeout = 30 * time.Second

func main() {
	appCfg := config.Load()
	log := logging.NewRoot(appCfg.Logging.Level, appCfg.Logging.Format, "api")

	if appCfg.Database.URL == "" {
		log.Error().Str("env", config.EnvDatabaseURL).Msg("required database URL missing")
		os.Exit(1)
	}
	temporalCfg, err := config.LoadTemporalFromEnv()
	if err != nil {
		log.Error().Err(err).Msg("temporal config")
		os.Exit(1)
	}

	dbCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	gdb, err := pgsql.Open(dbCtx, appCfg.Database)
	cancel()
	if err != nil {
		log.Error().Err(err).Msg("database open")
		os.Exit(1)
	}

	tc, err := client.Dial(client.Options{
		HostPort:  temporalCfg.Address,
		Namespace: temporalCfg.Namespace,
	})
	if err != nil {
		log.Error().Err(err).Msg("temporal dial")
		os.Exit(1)
	}
	defer tc.Close()

	getter := pgsql.NewGetter(gdb)
	slotRepo := slotstorage.NewRepository(getter)
	jobRepo := jobsstorage.NewRepository(getter)
	pipeRuns := pipelinestorage.NewRepository(getter)
	profileRepo := profilestorage.NewRepository(getter)
	profileSvc := profileimpl.NewService(profileRepo, pipeRuns, slotRepo, log)
	slotSvc := slotsimpl.NewService(slotRepo, jobRepo, pipeRuns, profileSvc, tc, temporalCfg.TaskQueue, slotsutils.DefaultIngestSourceIDs(), log)

	handler := publicapihandlers.NewHTTPHandler(appCfg.API.CORSAllowedOrigins, publicapihandlers.Deps{
		Slots:   slotSvc,
		Profile: profileSvc,
		Logger:  log,
	})

	srv := &http.Server{
		Addr:    appCfg.API.Listen,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info().
			Str("listen", appCfg.API.Listen).
			Str("temporal_address", temporalCfg.Address).
			Str("task_queue", temporalCfg.TaskQueue).
			Msg("listening")
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
