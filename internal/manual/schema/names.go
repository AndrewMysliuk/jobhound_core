// Package schema holds DTOs and frozen Temporal registration strings for manual slot runs (008).
package schema

// Parent manual orchestration workflow (to be registered in cmd/worker with this exact name).
const ManualSlotRunWorkflowName = "ManualSlotRunWorkflow"

// Persisted pipeline activities after splitting RunPersistedPipelineStages (008 / plan.md).
const (
	PersistPipelineStage2ActivityName = "PersistPipelineStage2Activity"
	PersistPipelineStage3ActivityName = "PersistPipelineStage3Activity"
)

// Parent manual workflow activities (DB only; workflow stays deterministic).
const (
	CreatePipelineRunActivityName        = "CreatePipelineRunActivity"
	ListSlotJobsPassedStage1ActivityName = "ListSlotJobsPassedStage1Activity"
)
