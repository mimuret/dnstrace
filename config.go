// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Option func(*config)

// Filter returns true when the message should be skipped from tracing.
type Filter func(m *dns.Msg) bool

type RequestFunc func(span trace.Span, m *dns.Msg, serverAddr string, clientAddr string)

type ResponseFunc func(span trace.Span, r *dns.Msg, rtt time.Duration, err error)

type config struct {
	// tracer is used to create spans for DNS messages.
	tracer trace.Tracer
	// propagator is used to extract and inject trace context from/to DNS messages.
	propagator propagation.TextMapPropagator
	// requestFuncs are used to update the span using the request message before processing is completed.
	requestFuncs []RequestFunc
	// responseFuncs are used to update the span using the response message after processing is completed.
	responseFuncs []ResponseFunc
	// filters are used to skip tracing for matched messages (filter returns true).
	filters []Filter
	// spanStartOpts are used to set options when starting a span.
	spanStartOpts []trace.SpanStartOption
}

func newConfig(opts []Option) *config {
	c := &config{
		tracer:        otel.GetTracerProvider().Tracer("dnstrace"),
		propagator:    otel.GetTextMapPropagator(),
		requestFuncs:  []RequestFunc{SetRequestAttributes},
		responseFuncs: []ResponseFunc{SetResponseAttributes},
		filters:       nil,
		spanStartOpts: nil,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithTracer Withs the tracer to use for spans created by the handler.
func WithTracer(tracer trace.Tracer) Option {
	return func(c *config) {
		c.tracer = tracer
	}
}

// WithPropagator Withs the propagator to use for extracting and injecting trace context from/to DNS messages.
func WithPropagator(propagator propagation.TextMapPropagator) Option {
	return func(c *config) {
		c.propagator = propagator
	}
}

// SetSpanStartOpts Withs the options to use when starting a span.
func SetSpanStartOpts(opts ...trace.SpanStartOption) Option {
	return func(c *config) {
		c.spanStartOpts = append(c.spanStartOpts, opts...)
	}
}

// WithFilters appends filters used to skip tracing for matched messages (filter returns true).
func WithFilters(filters ...Filter) Option {
	return func(c *config) {
		c.filters = append(c.filters, filters...)
	}
}

// WithRequestFuncs Withs functions to use for updating the span using the request message before processing is completed.
func WithRequestFuncs(f ...RequestFunc) Option {
	return func(c *config) {
		c.requestFuncs = append(c.requestFuncs, f...)
	}
}

// WithResponseFuncs Withs functions to use for updating the span using the response message after processing is completed.
func WithResponseFuncs(f ...ResponseFunc) Option {
	return func(c *config) {
		c.responseFuncs = append(c.responseFuncs, f...)
	}
}

// SetRequestFuncs Withs the functions to use for updating the span using the request message before processing is completed.
func SetRequestFuncs(fns ...RequestFunc) Option {
	return func(c *config) {
		c.requestFuncs = fns
	}
}

// SetResponseFuncs Withs the functions to use for updating the span using the response message after processing is completed.
func SetResponseFuncs(fns ...ResponseFunc) Option {
	return func(c *config) {
		c.responseFuncs = fns
	}
}
