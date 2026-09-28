// Package schema holds module-local types for the slots feature (API inputs, wire payloads).
package schema

import (
	jobschema "github.com/andrewmysliuk/jobhound_core/internal/jobs/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
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
	SlotID string
	Rules  []pipelineschema.Stage2Rule
}

// RunStage3Params is the input for starting stage 3 for a slot. MaxJobs must already be validated (1–100) by the caller.
type RunStage3Params struct {
	SlotID  string
	MaxJobs int
}

// JobListItemFromEntry maps storage list rows to GET …/stages/{1|2|3}/jobs items.
func JobListItemFromEntry(e jobschema.JobListEntry, includePipelineStatus bool, stage2Debug bool) apischema.JobListItem {
	hiringCountries := e.Job.HiringCountries
	if hiringCountries == nil {
		hiringCountries = []string{}
	}
	hiringRegions := e.Job.HiringRegions
	if hiringRegions == nil {
		hiringRegions = []string{}
	}
	item := apischema.JobListItem{
		JobID:           e.Job.ID,
		Title:           e.Job.Title,
		Company:         e.Job.Company,
		Description:     e.Job.Description,
		SourceID:        e.Job.Source,
		URL:             e.Job.URL,
		ApplyURL:        e.Job.ApplyURL,
		FirstSeenAt:     e.FirstSeenAt.UTC(),
		Stage3Rationale: e.Stage3Rationale,
		HiringCountries: hiringCountries,
		HiringRegions:   hiringRegions,
		HiringRaw:       e.Job.HiringRaw,
		Position:        e.Job.Position,
	}
	if includePipelineStatus && e.PipelineRunStatus != "" {
		st := e.PipelineRunStatus
		item.Status = &st
	}
	if !e.Job.PostedAt.IsZero() {
		t := e.Job.PostedAt.UTC()
		item.PostedAt = &t
	}
	if stage2Debug {
		hits := append([]pipeline.Stage2Hit(nil), e.Stage2Hits...)
		item.Hits = &hits
		b := e.Stage2Boost
		item.Stage2Boost = &b
	}
	return item
}

// ListJobsParams selects paginated jobs for stages 1–3. StatusQuery is empty (all rows) or exact stage2_status / stage3_status for stages 2–3; caller must reject status on stage 1.
type ListJobsParams struct {
	SlotID      string
	Stage       int
	Page        int
	Limit       int
	StatusQuery string
	Stage2Debug bool // when true, stage-2 job rows include stage2_hits and stage2_boost
}

// PatchJobBucketParams updates coarse outcome for stage 2 or 3. Stage must be 2 or 3.
type PatchJobBucketParams struct {
	SlotID string
	Stage  int
	JobID  string
	Bucket apischema.JobBucket
}
