package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeApplyURL_recruitee(t *testing.T) {
	raw := "https://eworgmbh.recruitee.com/o/ai-aiml-engineer-100-remote-mfd-33/c/new?utm_source=jobs&utm_medium=euremotejobs"
	got, err := NormalizeApplyURL(raw)
	require.NoError(t, err)
	require.Equal(t, "https://eworgmbh.recruitee.com/o/ai-aiml-engineer-100-remote-mfd-33", got)
}

func TestNormalizeApplyURL_otherHostUnchanged(t *testing.T) {
	raw := "https://ats.example.com/apply/1?ref=1"
	got, err := NormalizeApplyURL(raw)
	require.NoError(t, err)
	require.Equal(t, "https://ats.example.com/apply/1", got)
}

func TestRecruiteeCareersURLFromApply(t *testing.T) {
	got, ok := RecruiteeCareersURLFromApply("https://eworgmbh.recruitee.com/o/ai-aiml-engineer-100-remote-mfd-33/c/new")
	require.True(t, ok)
	require.Equal(t, "https://eworgmbh.recruitee.com/", got)
}

func TestRecruiteeNotFoundBody(t *testing.T) {
	require.True(t, recruiteeNotFoundBody(`<h1>We couldn't find this job</h1><p>This job doesn't exist or was removed.</p>`))
	require.False(t, recruiteeNotFoundBody(`<h1>Clean Energy Business Development Manager</h1>`))
}

