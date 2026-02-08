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
