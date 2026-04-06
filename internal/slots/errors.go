package slots

import "errors"

var (
	// ErrNotFound is returned when a slot id is absent.
	ErrNotFound = errors.New("slot not found")
	// ErrSlotLimitReached is returned when creating a slot would exceed the MVP cap (3).
	ErrSlotLimitReached = errors.New("slot limit reached")
	// ErrInvalidSlotName is returned when POST body name is missing or whitespace-only.
	ErrInvalidSlotName = errors.New("invalid slot name")
	// ErrStageAlreadyRunning is returned when POST …/stages/2|3/run finds that stage’s workflow still running.
	ErrStageAlreadyRunning = errors.New("stage already running")
	// ErrNoPipelineRun is returned when stage 3 is requested but the slot has no pipeline_runs row yet.
	ErrNoPipelineRun = errors.New("no pipeline run for slot")
	// ErrProfileRequired is returned when stage 3 runs but global profile text is empty.
	ErrProfileRequired = errors.New("profile text is required for stage 3")
	// ErrInvalidJobListQuery is returned for bad page/limit/status filter (009 GET …/jobs).
	ErrInvalidJobListQuery = errors.New("invalid job list query")
)
