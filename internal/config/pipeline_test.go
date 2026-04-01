package config

import (
	"testing"

	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
)

func TestLoadPipelineFromEnv_stage3Cap(t *testing.T) {
	t.Run("default matches pipeline utils constant", func(t *testing.T) {
		t.Setenv(EnvPipelineStage3MaxJobsPerRun, "")
		got := LoadPipelineFromEnv().Stage3MaxJobsPerRun
		if got != pipeutils.MaxStage3JobsPerPipelineRunExecution {
			t.Fatalf("default: got %d want %d", got, pipeutils.MaxStage3JobsPerPipelineRunExecution)
		}
	})
	t.Run("valid override", func(t *testing.T) {
		t.Setenv(EnvPipelineStage3MaxJobsPerRun, "3")
		if got := LoadPipelineFromEnv().Stage3MaxJobsPerRun; got != 3 {
			t.Fatalf("got %d want 3", got)
		}
	})
	t.Run("invalid falls back", func(t *testing.T) {
		t.Setenv(EnvPipelineStage3MaxJobsPerRun, "nope")
		got := LoadPipelineFromEnv().Stage3MaxJobsPerRun
		if got != defaultPipelineStage3MaxJobsPerRun {
			t.Fatalf("got %d want default %d", got, defaultPipelineStage3MaxJobsPerRun)
		}
	})
	t.Run("non positive falls back", func(t *testing.T) {
		t.Setenv(EnvPipelineStage3MaxJobsPerRun, "0")
		got := LoadPipelineFromEnv().Stage3MaxJobsPerRun
		if got != defaultPipelineStage3MaxJobsPerRun {
			t.Fatalf("got %d want default %d", got, defaultPipelineStage3MaxJobsPerRun)
		}
	})
	t.Run("clamp upper bound", func(t *testing.T) {
		t.Setenv(EnvPipelineStage3MaxJobsPerRun, "99999999")
		got := LoadPipelineFromEnv().Stage3MaxJobsPerRun
		if got != MaxPipelineStage3JobsPerRunUpperBound {
			t.Fatalf("got %d want %d", got, MaxPipelineStage3JobsPerRunUpperBound)
		}
	})
}
