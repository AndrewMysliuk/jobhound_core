package schema

import "fmt"

// RunJobStatus is a contract status string: stage 1 on jobs, stages 2–3 on pipeline_run_jobs.
type RunJobStatus string

const (
	RunJobPassedStage1   RunJobStatus = "PASSED_STAGE_1"
	RunJobRejectedStage2 RunJobStatus = "REJECTED_STAGE_2"
	RunJobPassedStage2   RunJobStatus = "PASSED_STAGE_2"
	RunJobUnknownStage2  RunJobStatus = "UNKNOWN_STAGE_2"
	RunJobPassedStage3   RunJobStatus = "PASSED_STAGE_3"
	RunJobRejectedStage3 RunJobStatus = "REJECTED_STAGE_3"
)

// Stage2ListFilterEligible is a GET …/stages/2/jobs ?status= value (not stored on rows): PASSED_STAGE_2 and UNKNOWN_STAGE_2.
const Stage2ListFilterEligible = "ELIGIBLE_STAGE_2"

func (s RunJobStatus) String() string { return string(s) }

func (s RunJobStatus) Equals(str string) bool { return string(s) == str }

func (s RunJobStatus) Pointer() *RunJobStatus { return &s }

// FromValue accepts all six status constants, including PASSED_STAGE_1.
func (s RunJobStatus) FromValue(str string) (RunJobStatus, error) {
	switch RunJobStatus(str) {
	case RunJobPassedStage1, RunJobRejectedStage2, RunJobPassedStage2,
		RunJobUnknownStage2, RunJobPassedStage3, RunJobRejectedStage3:
		return RunJobStatus(str), nil
	default:
		return "", fmt.Errorf("unknown RunJobStatus %q: valid values are %v", str, ValuesRunJobStatus())
	}
}

// ValuesRunJobStatus returns every status constant, including PASSED_STAGE_1.
func ValuesRunJobStatus() []RunJobStatus {
	return []RunJobStatus{
		RunJobPassedStage1,
		RunJobRejectedStage2,
		RunJobPassedStage2,
		RunJobUnknownStage2,
		RunJobPassedStage3,
		RunJobRejectedStage3,
	}
}

// FromStringRunJobStatus parses str into a RunJobStatus.
func FromStringRunJobStatus(str string) (RunJobStatus, error) {
	var z RunJobStatus
	return z.FromValue(str)
}

// Valid reports whether s is allowed on a pipeline_run_jobs row: stage 2–3 values, not PASSED_STAGE_1.
func (s RunJobStatus) Valid() bool {
	switch s {
	case RunJobRejectedStage2, RunJobPassedStage2, RunJobUnknownStage2, RunJobPassedStage3, RunJobRejectedStage3:
		return true
	default:
		return false
	}
}
