// Package schema holds module-local types for the slots feature (API inputs, wire payloads).
package schema

import (
	apischema "github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
	"github.com/google/uuid"
)

// CreateSlotParams is the input for creating a slot.
type CreateSlotParams struct {
	Name           string
	IdempotencyKey uuid.UUID
}

// CreateSlotResult is the outcome of POST /slots (201 on first create, 200 on idempotent replay).
type CreateSlotResult struct {
	Card    *apischema.SlotCard
	Created bool
}

// GetSlotParams is the input for loading one slot card.
type GetSlotParams struct {
	SlotID string
}

// DeleteSlotParams is the input for deleting a slot.
type DeleteSlotParams struct {
	SlotID string
}

// RunStage2Params is the input for starting stage 2 for a slot.
type RunStage2Params struct {
	SlotID  string
	Include []string
	Exclude []string
}

// RunStage3Params is the input for starting stage 3 for a slot. MaxJobs must already be validated (1–100) by the caller.
type RunStage3Params struct {
	SlotID  string
	MaxJobs int
}

// ListJobsParams selects paginated jobs for stages 1–3. StatusQuery is empty (all rows) or exact stage2_status / stage3_status for stages 2–3; caller must reject status on stage 1.
type ListJobsParams struct {
	SlotID      string
	Stage       int
	Page        int
	Limit       int
	StatusQuery string
}

// PatchJobBucketParams updates coarse outcome for stage 2 or 3. Stage must be 2 or 3.
type PatchJobBucketParams struct {
	SlotID string
	Stage  int
	JobID  string
	Bucket apischema.JobBucket
}
