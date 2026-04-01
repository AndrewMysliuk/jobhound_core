package pipeline

// RunJobStatus is a contract status string (007 pipeline-run-job-status.md §1.1–§1.2):
// stage 1 on jobs, stages 2–3 on pipeline_run_jobs.
type RunJobStatus string

const (
	RunJobPassedStage1   RunJobStatus = "PASSED_STAGE_1"
	RunJobRejectedStage2 RunJobStatus = "REJECTED_STAGE_2"
	RunJobPassedStage2   RunJobStatus = "PASSED_STAGE_2"
	RunJobPassedStage3   RunJobStatus = "PASSED_STAGE_3"
	RunJobRejectedStage3 RunJobStatus = "REJECTED_STAGE_3"
)

// Valid reports whether s is allowed for a pipeline_run_jobs row (§1.2 only).
func (s RunJobStatus) Valid() bool {
	switch s {
	case RunJobRejectedStage2, RunJobPassedStage2, RunJobPassedStage3, RunJobRejectedStage3:
		return true
	default:
		return false
	}
}
