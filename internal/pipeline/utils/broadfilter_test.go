package utils

import (
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/stretchr/testify/require"
)

func TestApplyBroadFilter_defaultWindowTenDays(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	inside := schema.Job{ID: "in", PostedAt: now.Add(-9 * 24 * time.Hour)}
	outside := schema.Job{ID: "out", PostedAt: now.Add(-11 * 24 * time.Hour)}

	got, err := ApplyBroadFilter(func() time.Time { return now }, pipeline.BroadFilterRules{}, []schema.Job{inside, outside})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "in", got[0].ID)
}
