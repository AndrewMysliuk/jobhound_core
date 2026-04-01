// Package schema holds module-local DTOs for pipeline: Temporal activity payloads (stage rules stay at module root).
package schema

import (
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

// PipelineStagesInput is the payload for RunPipelineStages.
type PipelineStagesInput struct {
	Jobs         []domain.Job
	BroadRules   pipeline.BroadFilterRules
	KeywordRules pipeline.KeywordRules
	Profile      string
}

// PipelineStagesOutput holds intermediate lists and the final scored jobs (stage 3).
type PipelineStagesOutput struct {
	AfterBroad    []domain.Job
	AfterKeywords []domain.Job
	Scored        []domain.ScoredJob
}

// PersistedPipelineStagesInput drives stage 1–3 with 007 per-run persistence and stage-3 cap.
type PersistedPipelineStagesInput struct {
	PipelineRunID int64
	Jobs          []domain.Job
	BroadRules    pipeline.BroadFilterRules
	KeywordRules  pipeline.KeywordRules
	Profile       string
	// BroadFilterKeyHash is optional SHA-256 hex of the canonical broad filter key (006); persisted on pipeline_runs when non-empty.
	BroadFilterKeyHash string
	// Stage3SentJobIDs is optional idempotency for Temporal retries: job IDs already sent to the scorer in this workflow execution.
	Stage3SentJobIDs []string
}

// PersistedPipelineStagesOutput mirrors [PipelineStagesOutput] and returns Stage3SentJobIDs for workflow retry bookkeeping.
type PersistedPipelineStagesOutput struct {
	AfterBroad       []domain.Job
	AfterKeywords    []domain.Job
	Scored           []domain.ScoredJob
	Stage3SentJobIDs []string
}
