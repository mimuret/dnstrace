// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Option func(*config)

type config struct {
	tracer      trace.Tracer
	propagators propagation.TextMapPropagator
}

func newConfig(opts []Option) *config {
	c := &config{
		tracer:      otel.GetTracerProvider().Tracer("dnstrace"),
		propagators: otel.GetTextMapPropagator(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithTracer(tracer trace.Tracer) Option {
	return func(c *config) {
		c.tracer = tracer
	}
}
