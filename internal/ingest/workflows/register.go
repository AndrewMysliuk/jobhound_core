package ingest_workflows

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingest_activities "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// WorkerDeps configures ingest workflow + activity when all required fields are set.
type WorkerDeps struct {
	Redis                  *ingest.RedisCoordinator
	Jobs                   jobs.JobRepository
	Watermarks             ingest.WatermarkStore
	Collectors             map[string]collectors.Collector
	DefaultExplicitRefresh bool
	// BroadRules: 004 stage-1 filter before SaveIngest (zero value = default 7-day UTC window per ApplyBroadFilter).
	BroadRules pipeline.BroadFilterRules
	// Clock optional; passed to ApplyBroadFilter (nil → time.Now).
	Clock func() time.Time
}

// Register registers IngestSourceWorkflow and RunIngestSource when Redis, Jobs, Watermarks, and Collectors are configured.
func Register(w worker.Worker, deps WorkerDeps) {
	if w == nil || deps.Redis == nil || deps.Jobs == nil || deps.Watermarks == nil || len(deps.Collectors) == 0 {
		return
	}
	ing := &ingest_activities.IngestActivities{
		Redis:                  deps.Redis,
		Jobs:                   deps.Jobs,
		Watermarks:             deps.Watermarks,
		Collectors:             deps.Collectors,
		DefaultExplicitRefresh: deps.DefaultExplicitRefresh,
		BroadRules:             deps.BroadRules,
		Clock:                  deps.Clock,
	}
	w.RegisterActivityWithOptions(ing.RunIngestSource, activity.RegisterOptions{
		Name: ingest_activities.RunIngestSourceActivityName,
	})
	w.RegisterWorkflowWithOptions(IngestSourceWorkflow, workflow.RegisterOptions{
		Name: IngestSourceWorkflowName,
	})
}
