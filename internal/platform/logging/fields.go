package logging

// JSON field keys for structured logs (specs/010-observability/contracts/logging.md).
const (
	FieldHandler    = "handler"
	FieldMethod     = "method"
	FieldWorkflow   = "workflow"
	FieldService    = "service"
	FieldRequestID  = "request_id"
	FieldWorkflowID = "workflow_id"
	FieldRunID      = "run_id"
	FieldSourceID   = "source_id"
)
