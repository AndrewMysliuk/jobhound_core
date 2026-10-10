package schema

// ProfileRunInput starts ProfileRunWorkflow. The profile YAML is not copied in.
type ProfileRunInput struct {
	ProfileID string
	RunID     int64
}

// ScoreProfileResult is the number of jobs evaluated for this run, including rejects.
type ScoreProfileResult struct {
	JobsScored int
}

// MarkRunFinishedInput closes a profile_runs row after ingest and scoring.
type MarkRunFinishedInput struct {
	RunID          int64
	Status         RunStatus
	JobsScored     int
	SourcesSkipped int
}

// RunCountersInput updates the running row so the client can poll progress.
type RunCountersInput struct {
	RunID          int64
	JobsScored     int
	SourcesSkipped int
}
