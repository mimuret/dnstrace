package dnstrace

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func newTracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer("github.com/mimuret/dnstrace")
}
