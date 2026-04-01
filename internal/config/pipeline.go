package config

import (
	"os"
	"strconv"
	"strings"
)

// Pipeline execution limits (specs/007-llm-policy-and-caps/contracts/environment.md).
const (
	EnvPipelineStage3MaxJobsPerRun = "JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN"
	// MaxPipelineStage3JobsPerRunUpperBound rejects absurd env values; does not change spec rules (at most N per execution).
	MaxPipelineStage3JobsPerRunUpperBound = 10_000
	// defaultPipelineStage3MaxJobsPerRun must match [github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils.MaxStage3JobsPerPipelineRunExecution] (007).
	defaultPipelineStage3MaxJobsPerRun = 5
)

// Pipeline holds optional overrides for pipeline-run execution (007).
type Pipeline struct {
	// Stage3MaxJobsPerRun is the cap N on distinct jobs entering stage 3 per pipeline-run execution (default 5, same as code constant in pipeline/utils).
	Stage3MaxJobsPerRun int
}

// LoadPipelineFromEnv reads JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN.
// Empty, invalid, or non-positive values use the code default; values above [MaxPipelineStage3JobsPerRunUpperBound] are clamped.
func LoadPipelineFromEnv() Pipeline {
	return Pipeline{Stage3MaxJobsPerRun: pipelineStage3MaxJobsPerRunFromEnv()}
}

func pipelineStage3MaxJobsPerRunFromEnv() int {
	raw := strings.TrimSpace(os.Getenv(EnvPipelineStage3MaxJobsPerRun))
	if raw == "" {
		return defaultPipelineStage3MaxJobsPerRun
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultPipelineStage3MaxJobsPerRun
	}
	if n > MaxPipelineStage3JobsPerRunUpperBound {
		return MaxPipelineStage3JobsPerRunUpperBound
	}
	return n
}
