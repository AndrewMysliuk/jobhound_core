package ingest_workflows

import (
	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	ingest_activities "github.com/andrewmysliuk/jobhound_core/internal/ingest/workflows/activities"
	"github.com/andrewmysliuk/jobhound_core/internal/jobs"
	"github.com/rs/zerolog"
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
	Log                    zerolog.Logger
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
		Log:                    deps.Log,
	}
	w.RegisterActivityWithOptions(ing.RunIngestSource, activity.RegisterOptions{
		Name: ingest_activities.RunIngestSourceActivityName,
	})
	w.RegisterWorkflowWithOptions(IngestSourceWorkflow, workflow.RegisterOptions{
		Name: IngestSourceWorkflowName,
	})
}
