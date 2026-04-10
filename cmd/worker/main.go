// Command worker runs the Temporal worker: registers ingest, jobs, pipeline, and manual workflows/activities.
package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/bootstrap"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/dou"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingest_workflows "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	jobs_workflows "github.com/andrewmysliuk/jobhound_core/internal/jobs/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/llm"
	"github.com/andrewmysliuk/jobhound_core/internal/llm/anthropic"
	llmmock "github.com/andrewmysliuk/jobhound_core/internal/llm/mock"
	manual_workflows "github.com/andrewmysliuk/jobhound_core/internal/manual/workflows"
	pipelinestorage "github.com/andrewmysliuk/jobhound_core/internal/pipeline/storage"
	pipeline_workflows "github.com/andrewmysliuk/jobhound_core/internal/pipeline/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/temporalopts"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	earlyLog := logging.NewRoot(config.DefaultLogLevel, config.DefaultLogFormat, "worker")
	cfg, err := config.LoadTemporalFromEnv()
	if err != nil {
		earlyLog.Error().Err(err).Msg("temporal config")
		os.Exit(1)
	}

	c, err := client.Dial(client.Options{
		HostPort:  cfg.Address,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		earlyLog.Error().Err(err).Msg("temporal dial")
		os.Exit(1)
	}
	defer c.Close()

	w := worker.New(c, cfg.TaskQueue, temporalopts.DefaultWorkerOptions())

	appCfg := config.Load()
	log := logging.NewRoot(appCfg.Logging.Level, appCfg.Logging.Format, "worker")
	var scorer llm.Scorer
	if strings.TrimSpace(appCfg.AnthropicAPIKey) != "" {
		as := anthropic.NewScorer(appCfg.AnthropicAPIKey, appCfg.AnthropicModel)
		sl := log.With().Str("component", "anthropic_scorer").Logger()
		as.Log = &sl
		scorer = as
	} else {
		scorer = llmmock.Scorer{}
	}

	actDeps := pipeline_workflows.ActivitiesDeps{
		Scorer:              scorer,
		Stage3MaxJobsPerRun: appCfg.Pipeline.Stage3MaxJobsPerRun,
		Log:                 log,
	}
	var (
		ingestRedis           *ingest.RedisCoordinator
		ingestWatermarks      ingest.WatermarkStore
		ingestCollectors      map[string]collectors.Collector
		ingestExplicitRefresh bool
	)
	if strings.TrimSpace(appCfg.Database.URL) != "" {
		dbCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		gdb, err := pgsql.Open(dbCtx, appCfg.Database)
		cancel()
		if err != nil {
			log.Error().Err(err).Msg("database open")
			os.Exit(1)
		}
		getter := pgsql.NewGetter(gdb)
		actDeps.JobsRepo = jobsstorage.NewRepository(getter)
		actDeps.RunRepo = pipelinestorage.NewRepository(getter)

		if ru := strings.TrimSpace(appCfg.Ingest.RedisURL); ru != "" {
			opt, err := redis.ParseURL(ru)
			if err != nil {
				log.Error().Err(err).Msg("redis url")
				os.Exit(1)
			}
			rdb := redis.NewClient(opt)
			defer func() { _ = rdb.Close() }()
			ingestRedis = ingest.NewRedisCoordinatorWithTTL(rdb, appCfg.Ingest.LockTTLSeconds, appCfg.Ingest.CooldownTTLSeconds)
			ingestWatermarks = ingest.NewGormWatermarkStore(getter)
			bootCtx, bcancel := context.WithTimeout(context.Background(), 2*time.Minute)
			er, wn, douColl, himColl, err := bootstrap.MVPCollectors(bootCtx, nil, appCfg.DataDir, appCfg.DouCollector, appCfg.HimalayasCollector)
			bcancel()
			if err != nil {
				log.Error().Err(err).Msg("collectors bootstrap")
				os.Exit(1)
			}
			ingestCollectors = map[string]collectors.Collector{
				ingest.NormalizeSourceID(europeremotely.SourceName): er,
				ingest.NormalizeSourceID(workingnomads.SourceName):  wn,
				ingest.NormalizeSourceID(dou.SourceName):            douColl,
			}
			if himColl != nil {
				ingestCollectors[ingest.NormalizeSourceID(himalayas.SourceName)] = himColl
			}
			ingestExplicitRefresh = appCfg.Ingest.ExplicitRefresh
		}
	}
	pipeline_workflows.RegisterActivities(w, actDeps)
	manual_workflows.Register(w, manual_workflows.WorkerDeps{
		Runs: actDeps.RunRepo,
		Jobs: actDeps.JobsRepo,
		Log:  log,
	})
	jobs_workflows.RegisterRetention(w, jobs_workflows.RetentionWorkerDeps{Jobs: actDeps.JobsRepo, Log: log})
	ingest_workflows.Register(w, ingest_workflows.WorkerDeps{
		Redis:                  ingestRedis,
		Jobs:                   actDeps.JobsRepo,
		Watermarks:             ingestWatermarks,
		Collectors:             ingestCollectors,
		DefaultExplicitRefresh: ingestExplicitRefresh,
		Log:                    log,
	})

	if actDeps.JobsRepo != nil && config.LoadJobRetentionScheduleUpsertFromEnv() {
		schedCtx, scancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := jobs_workflows.EnsureJobRetentionSchedule(schedCtx, c, cfg.TaskQueue)
		scancel()
		if err != nil {
			log.Warn().Err(err).Msg("job retention schedule (worker continues; use Temporal UI/CLI or bin/retention run)")
		}
	}

	log.Info().
		Str("task_queue", cfg.TaskQueue).
		Str("namespace", cfg.Namespace).
		Str("address", cfg.Address).
		Msg("polling")
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Error().Err(err).Msg("worker")
		os.Exit(1)
	}
}
