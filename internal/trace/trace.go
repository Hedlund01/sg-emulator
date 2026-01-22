package trace

import (
	"context"

	"github.com/google/uuid"
)

// contextKey is a private type for context keys to avoid collisions
type contextKey string

const traceIDKey contextKey = "trace_id"

// WithTraceID creates a new context with a trace ID
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// NewTraceID generates a new UUID v4 trace ID and adds it to the context
func NewTraceID(ctx context.Context) context.Context {
	traceID := uuid.New().String()
	return WithTraceID(ctx, traceID)
}

// GetTraceID extracts the trace ID from the context, returns empty string if not found
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}
