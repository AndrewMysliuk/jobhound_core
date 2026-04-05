package logging

import (
	"context"
	"strconv"

	"github.com/rs/zerolog"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota + 1
	ctxKeySlotID
	ctxKeyUserID
	ctxKeyPipelineRunID
)

// WithRequestID returns a child context carrying the HTTP request correlation id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// WithSlotID attaches a slot UUID string for EnrichWithContext.
func WithSlotID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeySlotID, id)
}

// WithUserID attaches a user id when known (MVP may omit).
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

// WithPipelineRunID attaches a pipeline run identifier as a string.
func WithPipelineRunID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyPipelineRunID, id)
}

// WithPipelineRunIDInt64 is a convenience for numeric run ids from APIs and workflows.
func WithPipelineRunIDInt64(ctx context.Context, id int64) context.Context {
	if id <= 0 {
		return ctx
	}
	return WithPipelineRunID(ctx, strconv.FormatInt(id, 10))
}

// EnrichWithContext copies correlation and domain ids from ctx onto the returned logger.
func EnrichWithContext(ctx context.Context, log zerolog.Logger) zerolog.Logger {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok && v != "" {
		log = log.With().Str(FieldRequestID, v).Logger()
	}
	if v, ok := ctx.Value(ctxKeySlotID).(string); ok && v != "" {
		log = log.With().Str(FieldSlotID, v).Logger()
	}
	if v, ok := ctx.Value(ctxKeyUserID).(string); ok && v != "" {
		log = log.With().Str(FieldUserID, v).Logger()
	}
	if v, ok := ctx.Value(ctxKeyPipelineRunID).(string); ok && v != "" {
		log = log.With().Str(FieldPipelineRunID, v).Logger()
	}
	return log
}
