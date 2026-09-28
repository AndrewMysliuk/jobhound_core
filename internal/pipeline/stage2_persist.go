package pipeline

// Stage2Hit is one fired rule persisted on pipeline_run_jobs.stage2_hits.
type Stage2Hit struct {
	RuleID  string `json:"rule_id"`
	Action  string `json:"action"`
	Matched string `json:"matched"`
}
