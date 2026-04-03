package schema

import ingestschema "github.com/andrewmysliuk/jobhound_core/internal/ingest/schema"

// ManualSlotRunAggregate is the API-facing aggregate returned by the parent manual workflow (contract §5).
type ManualSlotRunAggregate struct {
	TemporalWorkflowID string `json:"temporal_workflow_id"`
	TemporalRunID      string `json:"temporal_run_id"`
	PipelineRunID      *int64 `json:"pipeline_run_id,omitempty"`
	// Ingest maps source_id → per-source ingest summary when ingest steps ran.
	Ingest       map[string]ingestschema.IngestSourceOutput `json:"ingest,omitempty"`
	Stage2       *Stage2Aggregate                           `json:"stage2,omitempty"`
	Stage3       *Stage3Aggregate                           `json:"stage3,omitempty"`
	ErrorSummary string                                     `json:"error_summary,omitempty"`
}

// Stage2Aggregate holds stage-2 snapshot size hints (contract §5).
type Stage2Aggregate struct {
	Passed   int `json:"passed"`
	Rejected int `json:"rejected"`
}

// Stage3Aggregate holds stage-3 scoring summary (contract §5; cap normatively 20 per batch).
type Stage3Aggregate struct {
	Scored   int `json:"scored"`
	Cap      int `json:"cap,omitempty"`
	Passed   int `json:"passed"`
	Rejected int `json:"rejected"`
}
