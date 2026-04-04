// Command api runs the browser-facing JSON HTTP API (composition only).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	pipelinestorage "github.com/andrewmysliuk/jobhound_core/internal/pipeline/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	profileimpl "github.com/andrewmysliuk/jobhound_core/internal/profile/impl"
	profilestorage "github.com/andrewmysliuk/jobhound_core/internal/profile/storage"
	publicapihandlers "github.com/andrewmysliuk/jobhound_core/internal/publicapi/handlers"
	"github.com/andrewmysliuk/jobhound_core/internal/slots"
	slotsimpl "github.com/andrewmysliuk/jobhound_core/internal/slots/impl"
	slotstorage "github.com/andrewmysliuk/jobhound_core/internal/slots/storage"
	"go.temporal.io/sdk/client"
)

const shutdownTimeout = 30 * time.Second

func main() {
	appCfg := config.Load()

	if appCfg.Database.URL == "" {
		log.Printf("api: %s is required", config.EnvDatabaseURL)
		os.Exit(1)
	}
	temporalCfg, err := config.LoadTemporalFromEnv()
	if err != nil {
		log.Printf("api: %v", err)
		os.Exit(1)
	}

	dbCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	gdb, err := pgsql.Open(dbCtx, appCfg.Database)
	cancel()
	if err != nil {
		log.Printf("api: database: %v", err)
		os.Exit(1)
	}

	tc, err := client.Dial(client.Options{
		HostPort:  temporalCfg.Address,
		Namespace: temporalCfg.Namespace,
	})
	if err != nil {
		log.Printf("api: temporal dial: %v", err)
		os.Exit(1)
	}
	defer tc.Close()

	getter := pgsql.NewGetter(gdb)
	slotRepo := slotstorage.NewRepository(getter)
	jobRepo := jobsstorage.NewRepository(getter)
	pipeRuns := pipelinestorage.NewRepository(getter)
	profileRepo := profilestorage.NewRepository(getter)
	profileSvc := profileimpl.NewService(profileRepo, pipeRuns, slotRepo)
	slotSvc := slotsimpl.NewService(slotRepo, jobRepo, pipeRuns, profileSvc, tc, temporalCfg.TaskQueue, slots.DefaultIngestSourceIDs())

	handler := publicapihandlers.NewHTTPHandler(appCfg.API.CORSAllowedOrigins, publicapihandlers.Deps{
		Slots:   slotSvc,
		Profile: profileSvc,
	})

	srv := &http.Server{
		Addr:    appCfg.API.Listen,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("api: listening on %s (Temporal %s queue %q)", appCfg.API.Listen, temporalCfg.Address, temporalCfg.TaskQueue)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-errCh:
		log.Printf("api: %v", err)
		os.Exit(1)
	case <-quit:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("api: shutdown: %v", err)
			os.Exit(1)
		}
		log.Println("api: stopped")
	}
}
