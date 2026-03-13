// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"context"
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

func NewClient(operation string, base *dns.Client, opts ...Option) *Client {
	c := &Client{
		operation: operation,
		base:      base,
	}
	config := newConfig(opts)
	c.applyConfig(config)
	return c
}

type Client struct {
	operation   string
	base        *dns.Client
	tracer      trace.Tracer
	propagators propagation.TextMapPropagator
}

func (c *Client) applyConfig(config *config) {
	c.tracer = config.tracer
	c.propagators = config.propagators
}

func (c *Client) prepareSpan(span trace.Span, m *dns.Msg) {
	if len(m.Question) > 0 {
		qname := m.Question[0].Name
		qtype := dns.TypeToString[m.Question[0].Qtype]
		span.SetAttributes(
			semconv.DNSQuestionName(qname),
			attribute.String("dns.request.type", qtype),
		)
	}
}

func (c *Client) afterSpan(span trace.Span, r *dns.Msg, rtt time.Duration, err error) {
	if err != nil {
		return
	}
	if r == nil {
		return
	}
	rcode := dns.RcodeToString[r.Rcode]
	span.SetAttributes(
		attribute.String("dns.response.code", rcode),
	)
}

func (c *Client) ExchangeContext(ctx context.Context, m *dns.Msg, a string) (r *dns.Msg, rtt time.Duration, err error) {
	carrier := NewDNSMsgCarrier(m)
	c.propagators.Inject(ctx, carrier)
	ctx, span := c.tracer.Start(ctx, c.operation)
	defer span.End()
	c.prepareSpan(span, m)
	r, rtt, err = c.base.ExchangeContext(ctx, m, a)
	c.afterSpan(span, r, rtt, err)
	return r, rtt, err
}

func (c *Client) ExchangeWithConnContext(ctx context.Context, m *dns.Msg, conn *dns.Conn) (r *dns.Msg, rtt time.Duration, err error) {
	carrier := NewDNSMsgCarrier(m)
	c.propagators.Inject(ctx, carrier)
	ctx, span := c.tracer.Start(ctx, c.operation)
	defer span.End()
	c.prepareSpan(span, m)
	r, rtt, err = c.base.ExchangeWithConnContext(ctx, m, conn)
	c.afterSpan(span, r, rtt, err)
	return r, rtt, err
}
