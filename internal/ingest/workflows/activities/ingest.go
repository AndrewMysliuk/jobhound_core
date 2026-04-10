// Package ingest_activities hosts Temporal activities for ingest (collector fetch + Redis coordination).
package ingest_activities

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/logging"
	"github.com/google/uuid"
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
	// BroadRules are 004 stage-1 rules applied before persisting (007: PASSED_STAGE_1 only after broad stage 1 passes).
	BroadRules pipeline.BroadFilterRules
	// Clock is optional; used by ApplyBroadFilter for the default 7-day window when From/To unset; nil uses time.Now.
	Clock func() time.Time
	Log   zerolog.Logger
}

// RunIngestSource acquires the ingest lock, fetches via the 005 collector, applies broad stage 1 (004),
// upserts passing jobs via SaveIngest (007 PASSED_STAGE_1), updates watermark when incremental, sets cooldown.
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
	if in.SlotID == uuid.Nil {
		return nil, fmt.Errorf("ingest activity: slot_id is required")
	}
	id := ingest.NormalizeSourceID(in.SourceID)
	if id == "" {
		return nil, ingest.ErrEmptySourceID
	}
	ctx = logging.WithSlotID(ctx, in.SlotID.String())
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
	release, err := a.Redis.Begin(ctx, in.SlotID, id, explicit)
	if err != nil {
		log.Error().Err(err).Msg("redis begin")
		return nil, err
	}
	defer func() { _ = release(ctx) }()

	var list []schema.Job
	var usedIncr bool
	var nextCursor string
	slotQ := strings.TrimSpace(in.SlotSearchQuery)
	if slotQ != "" {
		if sf, ok := col.(collectors.SlotSearchFetcher); ok {
			list, err = sf.FetchWithSlotSearch(ctx, slotQ)
			if err != nil {
				log.Error().Err(err).Msg("fetch with slot search")
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
		cur, err := a.Watermarks.GetCursor(ctx, in.SlotID, id)
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

	filtered, err := pipeutils.ApplyBroadFilter(a.Clock, a.BroadRules, list)
	if err != nil {
		err = fmt.Errorf("ingest activity: broad filter: %w", err)
		log.Error().Err(err).Msg("broad filter")
		return nil, err
	}

	out := &ingestschema.IngestSourceOutput{
		UsedIncremental: usedIncr,
		JobsFilteredOut: len(list) - len(filtered),
	}
	for _, j := range filtered {
		skipped, err := a.Jobs.SaveIngest(ctx, j)
		if err != nil {
			log.Error().Err(err).Str("job_id", j.ID).Msg("save ingest")
			return nil, err
		}
		// 008: slot membership after ingest; SaveIngest alone sets PASSED_STAGE_1 (007) when the row is written/updated.
		if err := a.Jobs.UpsertSlotJob(ctx, in.SlotID, j.ID); err != nil {
			log.Error().Err(err).Str("job_id", j.ID).Msg("upsert slot job")
			return nil, err
		}
		if skipped {
			out.JobsSkipped++
		} else {
			out.JobsWritten++
		}
	}

	if usedIncr {
		if err := a.Watermarks.SetCursor(ctx, in.SlotID, id, nextCursor); err != nil {
			log.Error().Err(err).Msg("watermark set cursor")
			return nil, err
		}
		out.WatermarkAdvanced = true
	}

	if err := a.Redis.RecordSuccessfulIngest(ctx, in.SlotID, id); err != nil {
		log.Error().Err(err).Msg("record successful ingest")
		return nil, err
	}
	log.Debug().
		Int("jobs_written", out.JobsWritten).
		Int("jobs_skipped", out.JobsSkipped).
		Int("jobs_filtered_out", out.JobsFilteredOut).
		Bool("used_incremental", out.UsedIncremental).
		Msg("ingest done")
	return out, nil
}
