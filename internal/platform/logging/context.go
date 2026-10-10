package logging

import (
	"context"

	"github.com/rs/zerolog"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota + 1
)

// WithRequestID returns a child context carrying the HTTP request correlation id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// EnrichWithContext copies the request id from ctx onto the returned logger.
func EnrichWithContext(ctx context.Context, log zerolog.Logger) zerolog.Logger {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok && v != "" {
		log = log.With().Str(FieldRequestID, v).Logger()
	}
	return log
}
