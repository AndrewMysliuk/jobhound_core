package schema

// RunKind selects which steps a manual slot run executes (contracts/manual-workflow.md §3).
type RunKind string

const (
	RunKindIngestSources           RunKind = "INGEST_SOURCES"
	RunKindPipelineStage2          RunKind = "PIPELINE_STAGE2"
	RunKindPipelineStage3          RunKind = "PIPELINE_STAGE3"
	RunKindPipelineStage2Then3     RunKind = "PIPELINE_STAGE2_THEN_STAGE3"
	RunKindIngestThenPipeline      RunKind = "INGEST_THEN_PIPELINE"
	RunKindDeltaIngestThenPipeline RunKind = "DELTA_INGEST_THEN_PIPELINE"
)
