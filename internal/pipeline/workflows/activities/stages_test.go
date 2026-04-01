package pipeline_activities

import (
	"context"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	llmmock "github.com/andrewmysliuk/jobhound_core/internal/llm/mock"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
	"github.com/stretchr/testify/require"
)

func TestActivities_RunPipelineStages(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	jobs := []domain.Job{
		{
			ID: "a", Title: "Go Dev", Description: "backend",
			PostedAt: now.Add(-24 * time.Hour), Remote: ptr(true), CountryCode: "DE",
		},
		{
			ID: "b", Title: "Rust", Description: "systems",
			PostedAt: now.Add(-24 * time.Hour), Remote: ptr(true), CountryCode: "DE",
		},
	}
	a := &Activities{
		Clock:  func() time.Time { return now },
		Scorer: llmmock.Scorer{},
	}
	out, err := a.RunPipelineStages(context.Background(), pipelineschema.PipelineStagesInput{
		Jobs: jobs,
		BroadRules: pipeline.BroadFilterRules{
			RoleSynonyms:     []string{"go"},
			RemoteOnly:       true,
			CountryAllowlist: []string{"de"},
		},
		KeywordRules: pipeline.KeywordRules{Include: []string{"backend"}},
		Profile:      "cv",
	})
	require.NoError(t, err)
	require.Len(t, out.AfterBroad, 1)
	require.Len(t, out.AfterKeywords, 1)
	require.Len(t, out.Scored, 1)
	require.Equal(t, "a", out.Scored[0].Job.ID)
}

func ptr(b bool) *bool { return &b }
