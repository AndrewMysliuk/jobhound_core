package schema

import "time"

// StageState is the last finished or current run state for a stage (spec.md).
type StageState string

const (
	StageStateIdle      StageState = "idle"
	StageStateRunning   StageState = "running"
	StageStateSucceeded StageState = "succeeded"
	StageStateFailed    StageState = "failed"
)

// StageError is the structured error on a full stage card when state is failed (plan.md D3: object form).
type StageError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StageCompact is used in GET /api/v1/slots list items (state only).
type StageCompact struct {
	State StageState `json:"state"`
}

// StageFull is the full stage object on slot card and POST /slots (spec.md §stage object).
type StageFull struct {
	State      StageState  `json:"state"`
	StartedAt  *time.Time  `json:"started_at"`
	FinishedAt *time.Time  `json:"finished_at"`
	Error      *StageError `json:"error"`
}
