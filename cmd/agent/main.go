// Command agent runs optional local debug HTTP for collectors (composition only).
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/bootstrap"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/handlers/debughttp"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/rs/zerolog"
)

const debugHTTPShutdownTimeout = 30 * time.Second

func main() {
	debugHTTPAddr := flag.String("debug-http-addr", "", "if set, listen for local debug HTTP (GET /health, per-source POST /debug/collectors/…); overrides "+config.EnvDebugHTTPAddr)
	flag.Parse()

	ctx := context.Background()
	appCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	log := logging.NewRoot(appCfg.Logging.Level, appCfg.Logging.Format, "agent")
	addr := strings.TrimSpace(*debugHTTPAddr)
	if addr == "" {
		addr = strings.TrimSpace(appCfg.DebugHTTPAddr)
	}
	if addr == "" {
		fmt.Fprintln(os.Stderr, "jobhound_core: set -debug-http-addr or "+config.EnvDebugHTTPAddr)
		os.Exit(1)
	}

	er, wn, builtinColl, himColl, reColl, wwrColl, wfColl, vjColl, gcColl, err := bootstrap.MVPCollectors(ctx, nil, appCfg.DataDir, appCfg.BuiltinCollector, appCfg.HimalayasCollector, appCfg.Browser, appCfg.EuropeRemotely)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var wnConcrete *workingnomads.WorkingNomads
	if x, ok := wn.(*workingnomads.WorkingNomads); ok {
		wnConcrete = x
	}
	var erConcrete *europeremotely.EuropeRemotely
	if x, ok := er.(*europeremotely.EuropeRemotely); ok {
		erConcrete = x
	}
	var himConcrete *himalayas.Himalayas
	if x, ok := himColl.(*himalayas.Himalayas); ok {
		himConcrete = x
	}
	var builtinConcrete *builtin.BuiltIn
	if x, ok := builtinColl.(*builtin.BuiltIn); ok {
		builtinConcrete = x
	}
	var reConcrete *remotifyeurope.RemotifyEurope
	if x, ok := reColl.(*remotifyeurope.RemotifyEurope); ok {
		reConcrete = x
	}
	var wfConcrete *wellfound.Wellfound
	if x, ok := wfColl.(*wellfound.Wellfound); ok {
		wfConcrete = x
	}
	var vjConcrete *vuejobs.VueJobs
	if x, ok := vjColl.(*vuejobs.VueJobs); ok {
		vjConcrete = x
	}
	var gcConcrete *golangcafe.GolangCafe
	if x, ok := gcColl.(*golangcafe.GolangCafe); ok {
		gcConcrete = x
	}
	if err := runDebugHTTPServer(log, addr, er, wn, himColl, builtinColl, reColl, wwrColl, wfColl, vjColl, gcColl, wnConcrete, erConcrete, himConcrete, builtinConcrete, reConcrete, wfConcrete, vjConcrete, gcConcrete); err != nil {
		log.Error().Err(err).Msg("debug http")
		os.Exit(1)
	}
}

func runDebugHTTPServer(
	log zerolog.Logger, addr string,
	europeRemotely, workingNomads, himColl, builtinColl collectors.Collector,
	remotifyEurope, weWorkRemotely, wellfoundColl, vueJobs, golangCafe collectors.Collector,
	workingNomadsConcrete *workingnomads.WorkingNomads,
	europeRemotelyConcrete *europeremotely.EuropeRemotely,
	himalayasConcrete *himalayas.Himalayas,
	builtinConcrete *builtin.BuiltIn,
	remotifyEuropeConcrete *remotifyeurope.RemotifyEurope,
	wellfoundConcrete *wellfound.Wellfound,
	vueJobsConcrete *vuejobs.VueJobs,
	golangCafeConcrete *golangcafe.GolangCafe,
) error {
	srv := &http.Server{
		Addr: addr,
		Handler: debughttp.NewHTTPHandler(
			europeRemotely, workingNomads, himColl, builtinColl,
			remotifyEurope, weWorkRemotely, wellfoundColl, vueJobs, golangCafe,
			workingNomadsConcrete, europeRemotelyConcrete, himalayasConcrete, builtinConcrete,
			remotifyEuropeConcrete, wellfoundConcrete, vueJobsConcrete, golangCafeConcrete,
			log,
		),
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	log.Info().
		Str("listen", addr).
		Str("route_health", "GET /health").
		Str("route_europe", debughttp.PathEuropeRemotely).
		Str("route_working_nomads", debughttp.PathWorkingNomads).
		Str("route_himalayas", debughttp.PathHimalayas).
		Str("route_builtin", debughttp.PathBuiltin).
		Msg("debug http listening")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-errCh:
		return err
	case <-quit:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), debugHTTPShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("debug HTTP shutdown: %w", err)
		}
		log.Info().Msg("debug http stopped")
		return nil
	}
}
