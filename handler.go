// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"context"
	"net"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
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
	h.propagators = c.propagators
}

type handler struct {
	operation   string
	base        Handler
	tracer      trace.Tracer
	propagators propagation.TextMapPropagator
}

func (h *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var (
		qname string
		qtype string
		opts  []trace.SpanStartOption
	)
	ctx := context.Background()
	ctx = h.propagators.Extract(ctx, NewDNSMsgCarrier(r))
	if len(r.Question) > 0 {
		qname = r.Question[0].Name
		qtype = dns.TypeToString[r.Question[0].Qtype]
		opts = append(opts, trace.WithAttributes(
			semconv.DNSQuestionName(qname),
			attribute.String("dns.query.type", qtype),
		))
	}

	tracer := h.tracer

	if tracer == nil {
		tracer = otel.Tracer("dnstrace")
	}

	ctx, span := tracer.Start(ctx, h.operation, opts...)
	defer span.End()

	// Wrap w to use our ResponseWriter methods while also exposing
	// other interfaces that w may implement (http.CloseNotifier,
	// http.Flusher, http.Hijacker, http.Pusher, io.ReaderFrom).

	rww := &responseWriter{
		ResponseWriter: w,
	}

	h.base.ServeDNSWithContext(ctx, rww, r)

	if rww.msg != nil {
		rcode := dns.RcodeToString[rww.msg.Rcode]
		span.SetAttributes(
			attribute.String("dns.response.code", rcode),
		)
	}
}

var _ dns.ResponseWriter = (*responseWriter)(nil)

type responseWriter struct {
	dns.ResponseWriter
	msg *dns.Msg
}

func (w *responseWriter) LocalAddr() net.Addr {
	return w.ResponseWriter.LocalAddr()
}

func (w *responseWriter) RemoteAddr() net.Addr {
	return w.ResponseWriter.RemoteAddr()
}

func (w *responseWriter) WriteMsg(m *dns.Msg) (err error) {
	w.msg = m
	return w.ResponseWriter.WriteMsg(m)
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
