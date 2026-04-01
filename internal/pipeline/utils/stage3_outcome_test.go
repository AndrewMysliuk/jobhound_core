package utils

import (
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

func TestTerminalRunJobStatusFromScoredJob(t *testing.T) {
	if got := TerminalRunJobStatusFromScoredJob(domain.ScoredJob{Score: 59}); got != pipeline.RunJobRejectedStage3 {
		t.Fatalf("score 59: got %q", got)
	}
	if got := TerminalRunJobStatusFromScoredJob(domain.ScoredJob{Score: 60}); got != pipeline.RunJobPassedStage3 {
		t.Fatalf("score 60: got %q", got)
	}
}
