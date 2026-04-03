package schema

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	"github.com/stretchr/testify/require"
)

func TestManualSlotRunStartRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	from := "2026-01-15T10:00:00Z"
	to := "2026-02-01T23:59:59Z"
	req := ManualSlotRunStartRequest{
		SlotID:          "11111111-1111-4111-8111-111111111111",
		Kind:            RunKindPipelineStage2,
		SourceIDs:       nil,
		ExplicitRefresh: true,
		BroadRules: &BroadFilterRulesJSON{
			From:         &from,
			To:           &to,
			RoleSynonyms: []string{"engineer"},
			RemoteOnly:   true,
		},
		KeywordRules: &KeywordRulesJSON{
			Include: []string{"go"},
			Exclude: []string{"php"},
		},
		BroadFilterKeyHash: "abc",
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)

	var got ManualSlotRunStartRequest
	require.NoError(t, json.Unmarshal(b, &got))
	require.Equal(t, req, got)

	in, err := got.ToWorkflowInput()
	require.NoError(t, err)
	require.Equal(t, RunKindPipelineStage2, in.Kind)
	require.True(t, in.ExplicitRefresh)
	require.Equal(t, "abc", in.BroadFilterKeyHash)
	require.Equal(t, []string{"engineer"}, in.BroadRules.RoleSynonyms)
	require.True(t, in.BroadRules.RemoteOnly)
	require.Equal(t, pipeline.KeywordRules{Include: []string{"go"}, Exclude: []string{"php"}}, in.KeywordRules)
	require.NotNil(t, in.BroadRules.From)
	require.Equal(t, time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC), *in.BroadRules.From)
	require.NotNil(t, in.BroadRules.To)
	require.Equal(t, time.Date(2026, 2, 1, 23, 59, 59, 0, time.UTC), *in.BroadRules.To)
}

func TestManualSlotRunStartRequest_ToWorkflowInput_errors(t *testing.T) {
	t.Parallel()

	_, err := (ManualSlotRunStartRequest{SlotID: "not-a-uuid", Kind: RunKindPipelineStage2}).ToWorkflowInput()
	require.Error(t, err)

	bad := "nope"
	_, err = (ManualSlotRunStartRequest{
		SlotID:     "11111111-1111-4111-8111-111111111111",
		Kind:       RunKindPipelineStage2,
		BroadRules: &BroadFilterRulesJSON{From: &bad},
	}).ToWorkflowInput()
	require.Error(t, err)
}

func TestBroadFilterRulesJSON_ToPipeline_nil(t *testing.T) {
	t.Parallel()

	var j *BroadFilterRulesJSON
	got, err := j.ToPipeline()
	require.NoError(t, err)
	require.Equal(t, pipeline.BroadFilterRules{}, got)
}
