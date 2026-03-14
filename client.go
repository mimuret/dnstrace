// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"context"
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel/propagation"
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
	operation     string
	base          *dns.Client
	tracer        trace.Tracer
	propagator    propagation.TextMapPropagator
	requestFuncs  []RequestFunc
	responseFuncs []ResponseFunc

	filters       []Filter
	startSpanOpts []trace.SpanStartOption
}

func (c *Client) applyConfig(config *config) {
	c.tracer = config.tracer
	c.propagator = config.propagator
	c.requestFuncs = config.requestFuncs
	c.responseFuncs = config.responseFuncs
	c.filters = config.filters
	c.startSpanOpts = config.spanStartOpts
}

func (c *Client) ExchangeContext(ctx context.Context, m *dns.Msg, a string) (r *dns.Msg, rtt time.Duration, err error) {
	conn, err := c.base.DialContext(ctx, a)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = conn.Close() }()
	return c.ExchangeWithConnContext(ctx, m, conn)
}

func (c *Client) ExchangeWithConnContext(ctx context.Context, m *dns.Msg, conn *dns.Conn) (r *dns.Msg, rtt time.Duration, err error) {
	for _, filter := range c.filters {
		if filter(m) {
			return c.base.ExchangeWithConnContext(ctx, m, conn)
		}
	}
	ctx, span := c.tracer.Start(ctx, c.operation, c.startSpanOpts...)
	defer span.End()
	carrier := NewDNSMsgCarrier(m)
	c.propagator.Inject(ctx, carrier)
	for _, f := range c.requestFuncs {
		f(span, m, conn.RemoteAddr().String(), conn.LocalAddr().String())
	}
	r, rtt, err = c.base.ExchangeWithConnContext(ctx, m, conn)
	for _, f := range c.responseFuncs {
		f(span, r, rtt, err)
	}
	return r, rtt, err
}
