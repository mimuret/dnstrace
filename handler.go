// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"context"
	"net"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Handler interface {
	ServeDNSWithContext(ctx context.Context, w dns.ResponseWriter, r *dns.Msg)
}

func NewHandler(operation string, base Handler, opts ...Option) dns.Handler {
	h := &handler{
		operation: operation,
		base:      base,
	}
	c := newConfig(opts)
	h.applyConfig(c)
	return h
}

func (h *handler) applyConfig(c *config) {
	h.tracer = c.tracer
	h.propagator = c.propagator
	h.responseFuncs = c.responseFuncs
	h.requestFuncs = c.requestFuncs
	h.filters = c.filters
	h.spanStartOpts = c.spanStartOpts
}

type handler struct {
	operation     string
	base          Handler
	tracer        trace.Tracer
	propagator    propagation.TextMapPropagator
	requestFuncs  []RequestFunc
	responseFuncs []ResponseFunc
	filters       []Filter
	spanStartOpts []trace.SpanStartOption
}

func (h *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	for _, filter := range h.filters {
		if filter(r) {
			h.base.ServeDNSWithContext(context.Background(), w, r)
			return
		}
	}

	ctx := context.Background()
	ctx = h.propagator.Extract(ctx, NewDNSMsgCarrier(r))

	tracer := h.tracer

	if tracer == nil {
		tracer = newTracer()
	}

	ctx, span := tracer.Start(ctx, h.operation, h.spanStartOpts...)
	defer span.End()
	for _, f := range h.requestFuncs {
		f(span, r, w.LocalAddr().String(), w.RemoteAddr().String())
	}

	// Wrap w to use our ResponseWriter methods while also exposing
	// other interfaces that w may implement (http.CloseNotifier,
	// http.Flusher, http.Hijacker, http.Pusher, io.ReaderFrom).

	rww := &responseWriter{
		ResponseWriter: w,
	}
	h.base.ServeDNSWithContext(ctx, rww, r)

	for _, f := range h.responseFuncs {
		f(span, rww.msg, 0, rww.err)
	}

}

var _ dns.ResponseWriter = (*responseWriter)(nil)

type responseWriter struct {
	dns.ResponseWriter
	msg *dns.Msg
	err error
}

func (w *responseWriter) LocalAddr() net.Addr {
	return w.ResponseWriter.LocalAddr()
}

func (w *responseWriter) RemoteAddr() net.Addr {
	return w.ResponseWriter.RemoteAddr()
}

func (w *responseWriter) WriteMsg(m *dns.Msg) error {
	w.msg = m
	w.err = w.ResponseWriter.WriteMsg(m)
	return w.err
}

func (w *responseWriter) Write(m []byte) (int, error) {
	// todo: fetch msg from m
	return w.ResponseWriter.Write(m)
}

func (w *responseWriter) Close() error {
	return w.ResponseWriter.Close()
}

func (w *responseWriter) TsigStatus() error {
	return w.ResponseWriter.TsigStatus()
}

func (w *responseWriter) TsigTimersOnly(b bool) {
	w.ResponseWriter.TsigTimersOnly(b)
}

func (w *responseWriter) Hijack() {
	w.ResponseWriter.Hijack()
}
