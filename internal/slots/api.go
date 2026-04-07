package slots

import (
	"context"

	apischema "github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	slotschema "github.com/andrewmysliuk/jobhound_core/internal/slots/schema"
)

// API is the application surface for slot HTTP handlers (mock in handler tests).
type API interface {
	List(ctx context.Context) (apischema.SlotsListResponse, error)
	Create(ctx context.Context, p slotschema.CreateSlotParams) (slotschema.CreateSlotResult, error)
	Get(ctx context.Context, p slotschema.GetSlotParams) (*apischema.SlotCard, error)
	Delete(ctx context.Context, p slotschema.DeleteSlotParams) error
	RunStage2(ctx context.Context, p slotschema.RunStage2Params) (*apischema.StageRunAcceptedResponse, error)
	RunStage3(ctx context.Context, p slotschema.RunStage3Params) (*apischema.StageRunAcceptedResponse, error)
	ListJobs(ctx context.Context, p slotschema.ListJobsParams) (apischema.JobListResponse, error)
	PatchJobBucket(ctx context.Context, p slotschema.PatchJobBucketParams) (*apischema.PatchJobBucketResponse, error)
}
