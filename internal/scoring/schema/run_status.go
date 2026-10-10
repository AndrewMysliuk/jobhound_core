package schema

import "fmt"

// RunStatus is the lifecycle of one profile run.
type RunStatus string

const (
	RunStatusRunning   RunStatus = "RUNNING"
	RunStatusSucceeded RunStatus = "SUCCEEDED"
	RunStatusFailed    RunStatus = "FAILED"
)

func (r RunStatus) String() string { return string(r) }

func (r RunStatus) Equals(s string) bool { return string(r) == s }

func (r RunStatus) Pointer() *RunStatus { return &r }

func (r RunStatus) FromValue(s string) (RunStatus, error) {
	switch RunStatus(s) {
	case RunStatusRunning, RunStatusSucceeded, RunStatusFailed:
		return RunStatus(s), nil
	default:
		return "", fmt.Errorf("unknown RunStatus %q: valid values are %v", s, ValuesRunStatus())
	}
}

func ValuesRunStatus() []RunStatus {
	return []RunStatus{RunStatusRunning, RunStatusSucceeded, RunStatusFailed}
}

func FromStringRunStatus(s string) (RunStatus, error) {
	var z RunStatus
	return z.FromValue(s)
}
