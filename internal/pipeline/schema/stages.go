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

// PersistPipelineStage2Input drives in-memory stage 1–2 and persists REJECTED_STAGE_2 / PASSED_STAGE_2 per job
// that passed stage 1 (004 omission model: stage-1 drops get no pipeline_run_jobs row).
type PersistPipelineStage2Input struct {
	PipelineRunID int64
	Jobs          []domain.Job
	BroadRules    pipeline.BroadFilterRules
	KeywordRules  pipeline.KeywordRules
	// BroadFilterKeyHash is optional SHA-256 hex of the canonical broad filter key (006); persisted on pipeline_runs when non-empty.
	BroadFilterKeyHash string
}

// PersistPipelineStage2Output holds stage 1–2 job lists after persistence.
type PersistPipelineStage2Output struct {
	AfterBroad    []domain.Job
	AfterKeywords []domain.Job
}

// PersistPipelineStage3Input drives stage-3 scoring for one pipeline run (after stage 2 has persisted).
type PersistPipelineStage3Input struct {
	PipelineRunID int64
	Profile       string
	// Stage3SentJobIDs is optional idempotency for Temporal retries: job IDs already sent to the scorer in this workflow execution.
	Stage3SentJobIDs []string
}

// PersistPipelineStage3Output returns scored jobs and Stage3SentJobIDs for workflow retry bookkeeping.
type PersistPipelineStage3Output struct {
	Scored           []domain.ScoredJob
	Stage3SentJobIDs []string
}
