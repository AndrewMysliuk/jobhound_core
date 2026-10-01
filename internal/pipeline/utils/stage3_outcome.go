package utils

import (
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
)

// Stage3PassScoreMinimum is the inclusive minimum Scorer score (004 output) mapped to
// PASSED_STAGE_3. Lower scores map to REJECTED_STAGE_3 after a successful Score call
// (007 orchestration policy; scorer math unchanged).
const Stage3PassScoreMinimum = 60

// TerminalRunJobStatusFromScoredJob maps a successful stage-3 score to a per-run terminal status.
func TerminalRunJobStatusFromScoredJob(sj schema.ScoredJob) pipelineschema.RunJobStatus {
	if sj.Score >= Stage3PassScoreMinimum {
		return pipelineschema.RunJobPassedStage3
	}
	return pipelineschema.RunJobRejectedStage3
}
