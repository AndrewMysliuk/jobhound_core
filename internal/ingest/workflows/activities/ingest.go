// Package ingest_activities hosts Temporal activities for ingest (collector fetch + Redis coordination).
package ingest_activities

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
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
	id := ingest.NormalizeSourceID(in.SourceID)
	if id == "" {
		return nil, ingest.ErrEmptySourceID
	}
	col, ok := a.Collectors[id]
	if !ok || col == nil {
		return nil, fmt.Errorf("ingest activity: unknown source_id %q", id)
	}

	explicit := in.ExplicitRefresh || a.DefaultExplicitRefresh
	release, err := a.Redis.Begin(ctx, id, explicit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = release(ctx) }()

	var list []domain.Job
	var usedIncr bool
	var nextCursor string
	if inc, ok := col.(collectors.IncrementalCollector); ok {
		usedIncr = true
		cur, err := a.Watermarks.GetCursor(ctx, id)
		if err != nil {
			return nil, err
		}
		list, nextCursor, err = inc.FetchIncremental(ctx, cur)
		if err != nil {
			return nil, err
		}
	} else {
		list, err = col.Fetch(ctx)
		if err != nil {
			return nil, err
		}
	}

	filtered, err := pipeutils.ApplyBroadFilter(a.Clock, a.BroadRules, list)
	if err != nil {
		return nil, fmt.Errorf("ingest activity: broad filter: %w", err)
	}

	out := &ingestschema.IngestSourceOutput{
		UsedIncremental: usedIncr,
		JobsFilteredOut: len(list) - len(filtered),
	}
	for _, j := range filtered {
		skipped, err := a.Jobs.SaveIngest(ctx, j)
		if err != nil {
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
			return nil, err
		}
		out.WatermarkAdvanced = true
	}

	if err := a.Redis.RecordSuccessfulIngest(ctx, id); err != nil {
		return nil, err
	}
	return out, nil
}
