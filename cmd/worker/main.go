package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/bootstrap"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingest_workflows "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	jobs_workflows "github.com/andrewmysliuk/jobhound_core/internal/jobs/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/llm"
	"github.com/andrewmysliuk/jobhound_core/internal/llm/anthropic"
	llmmock "github.com/andrewmysliuk/jobhound_core/internal/llm/mock"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelinestorage "github.com/andrewmysliuk/jobhound_core/internal/pipeline/storage"
	pipeline_workflows "github.com/andrewmysliuk/jobhound_core/internal/pipeline/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	reference_workflows "github.com/andrewmysliuk/jobhound_core/internal/reference/workflows"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Job fetch runs from cmd/agent via internal/collectors/bootstrap; this worker runs stage activities on workflow-supplied jobs.
	cfg, err := config.LoadTemporalFromEnv()
	if err != nil {
		log.Printf("temporal worker: %v", err)
		os.Exit(1)
	}

	c, err := client.Dial(client.Options{
		HostPort:  cfg.Address,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		log.Printf("temporal worker: dial: %v", err)
		os.Exit(1)
	}
	defer c.Close()

	w := worker.New(c, cfg.TaskQueue, worker.Options{})

	appCfg := config.Load()
	var scorer llm.Scorer
	if strings.TrimSpace(appCfg.AnthropicAPIKey) != "" {
		scorer = anthropic.NewScorer(appCfg.AnthropicAPIKey, appCfg.AnthropicModel)
	} else {
		scorer = llmmock.Scorer{}
	}

	actDeps := pipeline_workflows.ActivitiesDeps{
		Scorer:              scorer,
		Stage3MaxJobsPerRun: appCfg.Pipeline.Stage3MaxJobsPerRun,
	}
	var (
		ingestRedis           *ingest.RedisCoordinator
		ingestWatermarks      ingest.WatermarkStore
		ingestCollectors      map[string]pipeline.Collector
		ingestExplicitRefresh bool
	)
	if strings.TrimSpace(appCfg.Database.URL) != "" {
		dbCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		gdb, err := pgsql.Open(dbCtx, appCfg.Database)
		cancel()
		if err != nil {
			log.Printf("temporal worker: database: %v", err)
			os.Exit(1)
		}
		getter := pgsql.NewGetter(gdb)
		actDeps.JobsRepo = jobsstorage.NewRepository(getter)
		actDeps.RunRepo = pipelinestorage.NewRepository(getter)

		if ru := strings.TrimSpace(appCfg.Ingest.RedisURL); ru != "" {
			opt, err := redis.ParseURL(ru)
			if err != nil {
				log.Printf("temporal worker: redis url: %v", err)
				os.Exit(1)
			}
			rdb := redis.NewClient(opt)
			defer func() { _ = rdb.Close() }()
			ingestRedis = ingest.NewRedisCoordinatorWithTTL(rdb, appCfg.Ingest.LockTTLSeconds, appCfg.Ingest.CooldownTTLSeconds)
			ingestWatermarks = ingest.NewGormWatermarkStore(getter)
			bootCtx, bcancel := context.WithTimeout(context.Background(), 2*time.Minute)
			er, wn, err := bootstrap.MVPCollectors(bootCtx, nil, appCfg.DataDir)
			bcancel()
			if err != nil {
				log.Printf("temporal worker: collectors: %v", err)
				os.Exit(1)
			}
			ingestCollectors = map[string]pipeline.Collector{
				ingest.NormalizeSourceID(europeremotely.SourceName): er,
				ingest.NormalizeSourceID(workingnomads.SourceName):  wn,
			}
			ingestExplicitRefresh = appCfg.Ingest.ExplicitRefresh
		}
	}
	pipeline_workflows.RegisterActivities(w, actDeps)
	jobs_workflows.RegisterRetention(w, jobs_workflows.RetentionWorkerDeps{Jobs: actDeps.JobsRepo})
	ingest_workflows.Register(w, ingest_workflows.WorkerDeps{
		Redis:                  ingestRedis,
		Jobs:                   actDeps.JobsRepo,
		Watermarks:             ingestWatermarks,
		Collectors:             ingestCollectors,
		DefaultExplicitRefresh: ingestExplicitRefresh,
	})

	if actDeps.JobsRepo != nil && config.LoadJobRetentionScheduleUpsertFromEnv() {
		schedCtx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := jobs_workflows.EnsureJobRetentionSchedule(schedCtx, c, cfg.TaskQueue)
		scancel()
		if err != nil {
			log.Printf("temporal worker: job retention schedule: %v (worker continues; use Temporal UI/CLI or bin/retention run)", err)
		}
	}

	reference_workflows.Register(w)

	log.Printf("temporal worker: polling queue %q namespace %q address %s", cfg.TaskQueue, cfg.Namespace, cfg.Address)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Printf("temporal worker: %v", err)
		os.Exit(1)
	}
}
