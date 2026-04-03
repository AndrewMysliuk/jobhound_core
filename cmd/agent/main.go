// Command agent runs the MVP pipeline or optional local debug HTTP for collectors (composition only).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/bootstrap"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/handlers/debughttp"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	llmmock "github.com/andrewmysliuk/jobhound_core/internal/llm/mock"
	manualschema "github.com/andrewmysliuk/jobhound_core/internal/manual/schema"
	manual_workflows "github.com/andrewmysliuk/jobhound_core/internal/manual/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline/impl"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline/mock"
)

const debugHTTPShutdownTimeout = 30 * time.Second

func main() {
	debugHTTPAddr := flag.String("debug-http-addr", "", "if set, listen for local debug HTTP (GET /health, per-source POST /debug/collectors/…); overrides "+config.EnvDebugHTTPAddr)
	temporalManual := flag.Bool("temporal-manual-slot-run", false, "dial Temporal (JOBHOUND_TEMPORAL_ADDRESS) and run ManualSlotRunWorkflow; prints JSON aggregate to stdout; use -manual-* flags")
	manualSlotID := flag.String("manual-slot-id", "", "slot UUID (required with -temporal-manual-slot-run)")
	manualRunKind := flag.String("manual-run-kind", string(manualschema.RunKindPipelineStage2), "ManualSlotRunWorkflow run kind (e.g. PIPELINE_STAGE2)")
	manualWorkflowID := flag.String("manual-workflow-id", "", "Temporal workflow ID (default: auto-generated)")
	manualSourceIDs := flag.String("manual-source-ids", "", "comma-separated source ids for ingest kinds")
	manualProfile := flag.String("manual-profile", "", "profile text when stage 3 runs")
	manualPipelineRunID := flag.Int64("manual-pipeline-run-id", 0, "pipeline run id for PIPELINE_STAGE3 (>0)")
	manualExplicitRefresh := flag.Bool("manual-explicit-refresh", false, "pass explicit refresh to ingest children when applicable")
	flag.Parse()

	ctx := context.Background()
	appCfg := config.Load()
	addr := strings.TrimSpace(*debugHTTPAddr)
	if addr == "" {
		addr = strings.TrimSpace(appCfg.DebugHTTPAddr)
	}

	if *temporalManual {
		if addr != "" {
			fmt.Fprintln(os.Stderr, "jobhound_core: use either -temporal-manual-slot-run or -debug-http-addr, not both")
			os.Exit(1)
		}
		runCtx, cancel := context.WithTimeout(ctx, manual_workflows.DefaultManualSlotRunWorkflowTimeout+time.Minute)
		defer cancel()
		err := runTemporalManualSlotRun(runCtx, temporalManualOpts{
			slotID:          *manualSlotID,
			runKind:         *manualRunKind,
			workflowID:      *manualWorkflowID,
			sourceIDs:       *manualSourceIDs,
			profile:         *manualProfile,
			pipelineRunID:   *manualPipelineRunID,
			explicitRefresh: *manualExplicitRefresh,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	er, wn, err := bootstrap.MVPCollectors(ctx, nil, appCfg.DataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if addr != "" {
		var wnConcrete *workingnomads.WorkingNomads
		if x, ok := wn.(*workingnomads.WorkingNomads); ok {
			wnConcrete = x
		}
		var erConcrete *europeremotely.EuropeRemotely
		if x, ok := er.(*europeremotely.EuropeRemotely); ok {
			erConcrete = x
		}
		if err := runDebugHTTPServer(addr, er, wn, wnConcrete, erConcrete); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	coll := bootstrap.MVPMulti(er, wn)
	p := &impl.Pipeline{
		Collector: coll,
		Scorer:    llmmock.Scorer{},
		Dedup:     mock.Dedup{},
		Notify:    mock.Notifier{},
	}
	if err := p.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "jobhound_core: noop pipeline run ok")
}

func runDebugHTTPServer(addr string, europeRemotely, workingNomads collectors.Collector, workingNomadsConcrete *workingnomads.WorkingNomads, europeRemotelyConcrete *europeremotely.EuropeRemotely) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: debughttp.NewHTTPHandler(europeRemotely, workingNomads, workingNomadsConcrete, europeRemotelyConcrete),
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	log.Printf("jobhound_core: debug HTTP on %s (GET /health, %s, %s); Ctrl+C to stop", addr, debughttp.PathEuropeRemotely, debughttp.PathWorkingNomads)

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
		log.Println("jobhound_core: debug HTTP stopped")
		return nil
	}
}
