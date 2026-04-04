package slots

import (
	"context"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
)

// API is the application surface for slot HTTP handlers (mock in handler tests).
type API interface {
	List(ctx context.Context) (schema.SlotsListResponse, error)
	Create(ctx context.Context, name string) (*schema.SlotCard, error)
	Get(ctx context.Context, slotID string) (*schema.SlotCard, error)
	Delete(ctx context.Context, slotID string) error
	RunStage2(ctx context.Context, slotID string, include, exclude []string) (*schema.StageRunAcceptedResponse, error)
	RunStage3(ctx context.Context, slotID string, maxJobs int) (*schema.StageRunAcceptedResponse, error)
	// ListJobs returns paginated jobs for stages 1–3. bucket is "" (all), "passed", or "failed" (stages 2–3 only); caller validates stage and bucket rules.
	ListJobs(ctx context.Context, slotID string, stage, page, limit int, bucket string) (schema.JobListResponse, error)
	// PatchJobBucket updates coarse outcome for stage 2 or 3 (009 PATCH). stage must be 2 or 3.
	PatchJobBucket(ctx context.Context, slotID string, stage int, jobID string, bucket schema.JobBucket) (*schema.PatchJobBucketResponse, error)
}
