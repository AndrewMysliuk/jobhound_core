package utils

import pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"

// EffectiveRunJobStatusFromRow is the combined outcome for stage-3 activity idempotency and GetRunJobStatus:
// terminal stage 3 when set, otherwise the stage-2 outcome.
func EffectiveRunJobStatusFromRow(stage2 string, stage3 *string) pipelineschema.RunJobStatus {
	if stage3 != nil && *stage3 != "" {
		return pipelineschema.RunJobStatus(*stage3)
	}
	return pipelineschema.RunJobStatus(stage2)
}
