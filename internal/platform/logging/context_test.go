package logging

import (
	"context"
	"testing"
)

func TestEnrichWithContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithSlotID(ctx, "slot-uuid")
	ctx = WithUserID(ctx, "user-1")
	ctx = WithPipelineRunID(ctx, "42")

	enriched := EnrichWithContext(ctx, Nop())
	enriched.Info().Msg("x")
}

func TestWithPipelineRunIDInt64_zeroNoop(t *testing.T) {
	ctx := WithPipelineRunIDInt64(context.Background(), 0)
	if _, ok := ctx.Value(ctxKeyPipelineRunID).(string); ok {
		t.Fatal("expected no pipeline_run_id for id<=0")
	}
}
