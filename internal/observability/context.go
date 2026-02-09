package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// DetachTraceContext creates a new context.Background() that carries the
// span context from the original request. This allows goroutines to
// create child spans linked to the HTTP request trace without inheriting
// its cancellation.
func DetachTraceContext(ctx context.Context) context.Context {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return context.Background()
	}
	return trace.ContextWithRemoteSpanContext(context.Background(), sc)
}

// DetachTraceContextFrom copies the trace span from src into baseCtx.
// Use this when you want goroutines to inherit a parent context's cancellation
// (e.g., SIGTERM) while carrying trace context from a different source (e.g.,
// an HTTP request context).
func DetachTraceContextFrom(src, baseCtx context.Context) context.Context {
	sc := trace.SpanContextFromContext(src)
	if !sc.IsValid() {
		return baseCtx
	}
	return trace.ContextWithRemoteSpanContext(baseCtx, sc)
}
