// Command worker runs the Temporal worker: registers ingest, job retention, and profile runs.
package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/bootstrap"
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
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingest_workflows "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	jobsstorage "github.com/andrewmysliuk/jobhound_core/internal/jobs/storage"
	jobs_workflows "github.com/andrewmysliuk/jobhound_core/internal/jobs/workflows"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/temporalopts"
	profilesimpl "github.com/andrewmysliuk/jobhound_core/internal/profiles/impl"
	scoringstorage "github.com/andrewmysliuk/jobhound_core/internal/scoring/storage"
	scoring_workflows "github.com/andrewmysliuk/jobhound_core/internal/scoring/workflows"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

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

	appCfg, err := config.Load()
	if err != nil {
		earlyLog.Error().Err(err).Msg("config")
		os.Exit(1)
	}
	log := logging.NewRoot(appCfg.Logging.Level, appCfg.Logging.Format, "worker")

	var (
		jobsRepo              jobs.JobRepository
		scoringRuns           *scoringstorage.Repository
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
		jobsRepo = jobsstorage.NewRepository(getter)
		scoringRuns = scoringstorage.NewRepository(getter)

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
			er, wn, builtinColl, himColl, reColl, wwrColl, wfColl, vjColl, gcColl, err := bootstrap.MVPCollectors(bootCtx, nil, appCfg.DataDir, appCfg.BuiltinCollector, appCfg.HimalayasCollector, appCfg.Browser, appCfg.EuropeRemotely)
			bcancel()
			if err != nil {
				log.Error().Err(err).Msg("collectors bootstrap")
				os.Exit(1)
			}
			ingestCollectors = ingestCollectorMap(er, wn, builtinColl, himColl, reColl, wwrColl, wfColl, vjColl, gcColl)
			ingestExplicitRefresh = appCfg.Ingest.ExplicitRefresh
		}
	}
	jobs_workflows.RegisterRetention(w, jobs_workflows.RetentionWorkerDeps{
		Jobs:             jobsRepo,
		Log:              log,
		JobRetentionDays: appCfg.JobRetentionDays,
	})
	ingest_workflows.Register(w, ingest_workflows.WorkerDeps{
		Redis:                  ingestRedis,
		Jobs:                   jobsRepo,
		Watermarks:             ingestWatermarks,
		Collectors:             ingestCollectors,
		DefaultExplicitRefresh: ingestExplicitRefresh,
		Log:                    log,
	})
	scoring_workflows.New(w, scoring_workflows.Deps{
		Profiles: profilesimpl.NewFileStore(appCfg.ProfilesDir, knownCollectorSourceIDs),
		Runs:     scoringRuns,
		Jobs:     jobsRepo,
		Log:      log,
	})

	if jobsRepo != nil && config.LoadJobRetentionScheduleUpsertFromEnv() {
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

// ingestCollectorMap keys are the collectors this process built.
// himalayas is omitted when nil; golang_cafe is omitted when the rod fetcher is nil.
func ingestCollectorMap(
	er, wn, builtinColl, himColl,
	reColl, wwrColl, wfColl, vjColl, gcColl collectors.Collector,
) map[string]collectors.Collector {
	m := map[string]collectors.Collector{
		ingest.NormalizeSourceID(europeremotely.SourceName): er,
		ingest.NormalizeSourceID(workingnomads.SourceName):  wn,
		ingest.NormalizeSourceID(builtin.SourceName):        builtinColl,
		ingest.NormalizeSourceID(remotifyeurope.SourceName): reColl,
		ingest.NormalizeSourceID(weworkremotely.SourceName): wwrColl,
		ingest.NormalizeSourceID(wellfound.SourceName):      wfColl,
		ingest.NormalizeSourceID(vuejobs.SourceName):        vjColl,
	}
	if himColl != nil {
		m[ingest.NormalizeSourceID(himalayas.SourceName)] = himColl
	}
	if gcColl != nil {
		m[ingest.NormalizeSourceID(golangcafe.SourceName)] = gcColl
	}
	return m
}
