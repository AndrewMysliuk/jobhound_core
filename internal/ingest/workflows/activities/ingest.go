// Package ingest_activities hosts Temporal activities for ingest (collector fetch + Redis coordination).
package ingest_activities

import (
	"context"
	"fmt"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/rs/zerolog"
)

// RunIngestSourceActivityName is the registered Temporal activity for per-source ingest (006).
const RunIngestSourceActivityName = "RunIngestSourceActivity"

// IngestActivities runs collector fetch + job upsert behind Redis lock/cooldown (006).
type IngestActivities struct {
	Redis                  *ingest.RedisCoordinator
	Jobs                   jobs.JobRepository
	Watermarks             ingest.WatermarkStore
	Collectors             map[string]collectors.Collector
	DefaultExplicitRefresh bool
	Log                    zerolog.Logger
}

// RunIngestSource acquires the ingest lock, fetches via the collector,
// upserts jobs via SaveIngest, updates the watermark when incremental, and sets cooldown.
func (a *IngestActivities) RunIngestSource(ctx context.Context, in ingestschema.IngestSourceInput) (*ingestschema.IngestSourceOutput, error) {
	if a == nil || a.Redis == nil {
		return nil, ingest.ErrNilRedisClient
	}
	if a.Jobs == nil {
		return nil, fmt.Errorf("ingest activity: Jobs repository is required")
	}
	if a.Watermarks == nil {
		return nil, fmt.Errorf("ingest activity: Watermarks store is required")
	}
	id := ingest.NormalizeSourceID(in.SourceID)
	if id == "" {
		return nil, ingest.ErrEmptySourceID
	}
	log := logging.EnrichWithContext(ctx, logging.LoggerWithActivity(ctx, a.Log, RunIngestSourceActivityName)).
		With().Str(logging.FieldSourceID, id).Logger()
	col, ok := a.Collectors[id]
	if !ok || col == nil {
		err := fmt.Errorf("ingest activity: unknown source_id %q", id)
		log.Error().Err(err).Msg("unknown source")
		return nil, err
	}

	log.Debug().Msg("ingest start")

	explicit := in.ExplicitRefresh || a.DefaultExplicitRefresh
	release, err := a.Redis.Begin(ctx, id, in.Query, explicit)
	if err != nil {
		log.Error().Err(err).Msg("redis begin")
		return nil, err
	}
	defer func() { _ = release(ctx) }()

	var list []schema.Job
	var usedIncr bool
	var nextCursor string
	query := strings.TrimSpace(in.Query)
	if query != "" {
		if sf, ok := col.(collectors.QueryFetcher); ok {
			list, err = sf.FetchWithQuery(ctx, query)
			if err != nil {
				log.Error().Err(err).Msg("fetch with query")
				return nil, err
			}
		} else {
			list, err = col.Fetch(ctx)
			if err != nil {
				log.Error().Err(err).Msg("fetch")
				return nil, err
			}
		}
	} else if inc, ok := col.(collectors.IncrementalCollector); ok {
		usedIncr = true
		cur, err := a.Watermarks.GetCursor(ctx, id)
		if err != nil {
			log.Error().Err(err).Msg("watermark get cursor")
			return nil, err
		}
		list, nextCursor, err = inc.FetchIncremental(ctx, cur)
		if err != nil {
			log.Error().Err(err).Msg("fetch incremental")
			return nil, err
		}
	} else {
		list, err = col.Fetch(ctx)
		if err != nil {
			log.Error().Err(err).Msg("fetch")
			return nil, err
		}
	}

	out := &ingestschema.IngestSourceOutput{
		UsedIncremental: usedIncr,
	}
	for _, j := range list {
		skipped, err := a.Jobs.SaveIngest(ctx, j)
		if err != nil {
			log.Error().Err(err).Str("job_id", j.ID).Msg("save ingest")
			return nil, err
		}
		if skipped {
			out.JobsSkipped++
		} else {
			out.JobsWritten++
		}
	}

	if usedIncr {
		if err := a.Watermarks.SetCursor(ctx, id, nextCursor); err != nil {
			log.Error().Err(err).Msg("watermark set cursor")
			return nil, err
		}
		out.WatermarkAdvanced = true
	}

	if err := a.Redis.RecordSuccessfulIngest(ctx, id, in.Query); err != nil {
		log.Error().Err(err).Msg("record successful ingest")
		return nil, err
	}
	log.Debug().
		Int("jobs_written", out.JobsWritten).
		Int("jobs_skipped", out.JobsSkipped).
		Bool("used_incremental", out.UsedIncremental).
		Msg("ingest done")
	return out, nil
}
