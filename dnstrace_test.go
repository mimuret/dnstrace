// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/nettest"
)

var (
	th = &testHandler{}
)

type testHandler struct {
	req *dns.Msg
	ctx context.Context
}

func (s *testHandler) ServeDNSWithContext(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
	s.req = r
	s.ctx = ctx
	m := new(dns.Msg)
	m.SetReply(r)
	a, _ := dns.NewRR("example.com. IN A 127.0.0.1")
	m.Answer = append(m.Answer, a)
	_ = w.WriteMsg(m)
}

func startServerWithOptions(opts ...Option) (*dns.Server, net.Listener, error) {
	ln, err := nettest.NewLocalListener("tcp")
	if err != nil {
		return nil, nil, err
	}

	server := dns.Server{
		Listener: ln,
		Handler:  NewHandler("dnstrace-test", th, opts...),
	}
	go func() {
		_ = server.ActivateAndServe()
	}()
	return &server, ln, nil
}

func setupOTel(t *testing.T, tp trace.TracerProvider) {
	t.Helper()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(newPropagator())
}

func setupStdoutOTel(t *testing.T) *sdktrace.TracerProvider {
	t.Helper()
	tp, err := newTracerProvider()
	if err != nil {
		t.Fatal(err)
	}
	setupOTel(t, tp)
	return tp
}

func setupRecordedOTel(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	setupOTel(t, tp)
	return tp, sr
}

func startTestServer(t *testing.T, opts ...Option) net.Listener {
	t.Helper()
	server, ln, err := startServerWithOptions(opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = server.Shutdown()
	})
	return ln
}

func newTestClient(tp *sdktrace.TracerProvider, opts ...Option) *Client {
	baseOpts := []Option{WithTracer(tp.Tracer("client"))}
	baseOpts = append(baseOpts, opts...)
	return NewClient("sent-query", &dns.Client{Net: "tcp"}, baseOpts...)
}

// TestHandler verifies that the handler itself does not add EDNS0 trace options to responses.
func TestHandler(t *testing.T) {
	_ = setupStdoutOTel(t)
	ln := startTestServer(t, WithTracer(otel.Tracer("dnstrace-test")))
	client := &dns.Client{
		Net: "tcp",
	}
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	r, _, err := client.Exchange(m, ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Extra) != 0 {
		t.Fatal("exist extra")
	}
	if traceOpt := GetEDNS0_TRACE(r); traceOpt != nil {
		t.Fatal("EDNS0_TRACE")
	}
}

// TestClient verifies that the client injects EDNS0 trace context and preserves tracestate.
func TestClient(t *testing.T) {
	tracerProvider := setupStdoutOTel(t)
	ln := startTestServer(t, WithTracer(otel.Tracer("dnstrace-test")))

	client := NewClient("sent-query", &dns.Client{
		Net: "tcp",
	})
	// without opt
	m1 := new(dns.Msg)
	m1.SetQuestion("example.com.", dns.TypeA)
	m2 := new(dns.Msg)
	m2.SetQuestion("example.com.", dns.TypeA)
	m2.SetEdns0(4096, true)
	m3 := new(dns.Msg)
	m3.SetQuestion("example.com.", dns.TypeA)
	m3.SetEdns0(4096, true)
	SetEDNS0_TRACE(m3, &EDNS0_TRACE{
		Version:    0x01,
		TraceID:    [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
		SpanID:     [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		TraceFlags: 0x01,
		Tracestate: []byte("key1=value1,key2=value2"),
	})
	testcase := []struct {
		m          *dns.Msg
		Tracestate string
	}{
		{
			m: m1,
		},
		{
			m: m2,
		},
		{
			m:          m3,
			Tracestate: "key1=value1,key2=value2",
		},
	}

	ctx, span := tracerProvider.Tracer("dnstrace-test").Start(context.Background(), "sent-query")
	defer span.End()

	for _, tc := range testcase {
		_, _, err := client.ExchangeContext(ctx, tc.m, ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		if len(th.req.Extra) == 0 {
			t.Fatal("extra not found")
		}
		traceOpt := GetEDNS0_TRACE(th.req)
		if traceOpt == nil {
			t.Fatal("EDNS0_TRACE not found")
		}
		if th.ctx == nil {
			t.Fatal("ctx is nil")
		}
		serverSpan := trace.SpanFromContext(th.ctx)
		if serverSpan.SpanContext().TraceID().String() != traceOpt.TraceID.String() {
			t.Fatalf("trace id not match: %s != %s", serverSpan.SpanContext().TraceID().String(), traceOpt.TraceID.String())
		}
		if tc.Tracestate != "" {
			if string(traceOpt.Tracestate) != tc.Tracestate {
				t.Fatalf("tracestate not match: %s != %s", string(traceOpt.Tracestate), tc.Tracestate)
			}
		}
	}
}

// TestClientFilterSkipsTraceContextAndSetsDefaultAttributes verifies filter skip behavior and default attributes for traced requests.
func TestClientFilterSkipsTraceContextAndSetsDefaultAttributes(t *testing.T) {
	tp, sr := setupRecordedOTel(t)
	ln := startTestServer(t, WithTracer(tp.Tracer("server")))

	client := newTestClient(tp,
		AppendFilter(func(m *dns.Msg) bool {
			return len(m.Question) > 0 && m.Question[0].Name == "filtered.example."
		}),
	)

	ctx, span := tp.Tracer("test").Start(context.Background(), "parent")

	filtered := new(dns.Msg)
	filtered.SetQuestion("filtered.example.", dns.TypeA)
	_, _, err := client.ExchangeContext(ctx, filtered, ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if traceOpt := GetEDNS0_TRACE(th.req); traceOpt != nil {
		t.Fatal("filtered request must not have EDNS0_TRACE")
	}

	unfiltered := new(dns.Msg)
	unfiltered.SetQuestion("example.com.", dns.TypeA)
	_, _, err = client.ExchangeContext(ctx, unfiltered, ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if traceOpt := GetEDNS0_TRACE(th.req); traceOpt == nil {
		t.Fatal("unfiltered request must have EDNS0_TRACE")
	}

	span.End()

	clientSpans := findSpansByName(sr.Ended(), "sent-query")
	if len(clientSpans) != 1 {
		t.Fatalf("expected 1 client span for unfiltered request, got %d", len(clientSpans))
	}
	assertHasDefaultAttributes(t, clientSpans[0])
}

// TestHandlerFilterSkipsTraceContextAndSetsDefaultAttributes verifies handler-side filter skip behavior and default attributes for traced requests.
func TestHandlerFilterSkipsTraceContextAndSetsDefaultAttributes(t *testing.T) {
	tp, sr := setupRecordedOTel(t)

	ln := startTestServer(t,
		WithTracer(tp.Tracer("handler")),
		AppendFilter(func(m *dns.Msg) bool {
			return len(m.Question) > 0 && m.Question[0].Name == "filtered.example."
		}),
	)

	client := newTestClient(tp)
	ctx, span := tp.Tracer("test").Start(context.Background(), "parent")

	filtered := new(dns.Msg)
	filtered.SetQuestion("filtered.example.", dns.TypeA)
	_, _, err := client.ExchangeContext(ctx, filtered, ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if trace.SpanFromContext(th.ctx).SpanContext().IsValid() {
		t.Fatal("filtered request must not carry trace context into handler context")
	}

	unfiltered := new(dns.Msg)
	unfiltered.SetQuestion("example.com.", dns.TypeA)
	_, _, err = client.ExchangeContext(ctx, unfiltered, ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if !trace.SpanFromContext(th.ctx).SpanContext().IsValid() {
		t.Fatal("unfiltered request must carry trace context into handler context")
	}

	span.End()

	handlerSpans := findSpansByName(sr.Ended(), "dnstrace-test")
	if len(handlerSpans) != 1 {
		t.Fatalf("expected 1 handler span for unfiltered request, got %d", len(handlerSpans))
	}
	assertHasDefaultAttributes(t, handlerSpans[0])
}

// TestClientFiltersShortCircuitAndAllPass verifies client filter short-circuit behavior and non-matching tracing behavior.
func TestClientFiltersShortCircuitAndAllPass(t *testing.T) {
	t.Run("short-circuit on true", func(t *testing.T) {
		tp, sr := setupRecordedOTel(t)
		ln := startTestServer(t, WithTracer(tp.Tracer("server")))

		firstCalled := 0
		secondCalled := 0
		client := newTestClient(tp,
			AppendFilter(func(m *dns.Msg) bool {
				firstCalled++
				return true
			}),
			AppendFilter(func(m *dns.Msg) bool {
				secondCalled++
				return true
			}),
		)

		ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
		defer span.End()

		m := new(dns.Msg)
		m.SetQuestion("example.com.", dns.TypeA)
		_, _, err := client.ExchangeContext(ctx, m, ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		if firstCalled != 1 {
			t.Fatalf("expected first filter to be called once, got %d", firstCalled)
		}
		if secondCalled != 0 {
			t.Fatalf("expected second filter to not be called, got %d", secondCalled)
		}
		if traceOpt := GetEDNS0_TRACE(th.req); traceOpt != nil {
			t.Fatal("short-circuited request must not have EDNS0_TRACE")
		}

		clientSpans := findSpansByName(sr.Ended(), "sent-query")
		if len(clientSpans) != 0 {
			t.Fatalf("expected 0 client spans for short-circuited request, got %d", len(clientSpans))
		}
	})

	t.Run("all filters false", func(t *testing.T) {
		tp, sr := setupRecordedOTel(t)
		ln := startTestServer(t, WithTracer(tp.Tracer("server")))

		firstCalled := 0
		secondCalled := 0
		client := newTestClient(tp,
			AppendFilter(func(m *dns.Msg) bool {
				firstCalled++
				return false
			}),
			AppendFilter(func(m *dns.Msg) bool {
				secondCalled++
				return false
			}),
		)

		ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
		defer span.End()

		m := new(dns.Msg)
		m.SetQuestion("example.com.", dns.TypeA)
		_, _, err := client.ExchangeContext(ctx, m, ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		if firstCalled != 1 || secondCalled != 1 {
			t.Fatalf("expected both filters to be called once, got first=%d second=%d", firstCalled, secondCalled)
		}
		if traceOpt := GetEDNS0_TRACE(th.req); traceOpt == nil {
			t.Fatal("non-matching request must have EDNS0_TRACE")
		}

		clientSpans := findSpansByName(sr.Ended(), "sent-query")
		if len(clientSpans) != 1 {
			t.Fatalf("expected 1 client span for non-matching request, got %d", len(clientSpans))
		}
		assertHasDefaultAttributes(t, clientSpans[0])
	})
}

// TestHandlerFiltersShortCircuitAndAllPass verifies handler filter short-circuit behavior and non-matching tracing behavior.
func TestHandlerFiltersShortCircuitAndAllPass(t *testing.T) {
	t.Run("short-circuit on true", func(t *testing.T) {
		tp, sr := setupRecordedOTel(t)

		firstCalled := 0
		secondCalled := 0
		ln := startTestServer(t,
			WithTracer(tp.Tracer("handler")),
			AppendFilter(func(m *dns.Msg) bool {
				firstCalled++
				return true
			}),
			AppendFilter(func(m *dns.Msg) bool {
				secondCalled++
				return true
			}),
		)

		client := newTestClient(tp)
		ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
		defer span.End()

		m := new(dns.Msg)
		m.SetQuestion("example.com.", dns.TypeA)
		_, _, err := client.ExchangeContext(ctx, m, ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		if firstCalled != 1 {
			t.Fatalf("expected first filter to be called once, got %d", firstCalled)
		}
		if secondCalled != 0 {
			t.Fatalf("expected second filter to not be called, got %d", secondCalled)
		}
		if trace.SpanFromContext(th.ctx).SpanContext().IsValid() {
			t.Fatal("short-circuited request must not carry trace context into handler context")
		}

		handlerSpans := findSpansByName(sr.Ended(), "dnstrace-test")
		if len(handlerSpans) != 0 {
			t.Fatalf("expected 0 handler spans for short-circuited request, got %d", len(handlerSpans))
		}
	})

	t.Run("all filters false", func(t *testing.T) {
		tp, sr := setupRecordedOTel(t)

		firstCalled := 0
		secondCalled := 0
		ln := startTestServer(t,
			WithTracer(tp.Tracer("handler")),
			AppendFilter(func(m *dns.Msg) bool {
				firstCalled++
				return false
			}),
			AppendFilter(func(m *dns.Msg) bool {
				secondCalled++
				return false
			}),
		)

		client := newTestClient(tp)
		ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
		defer span.End()

		m := new(dns.Msg)
		m.SetQuestion("example.com.", dns.TypeA)
		_, _, err := client.ExchangeContext(ctx, m, ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		if firstCalled != 1 || secondCalled != 1 {
			t.Fatalf("expected both filters to be called once, got first=%d second=%d", firstCalled, secondCalled)
		}
		if !trace.SpanFromContext(th.ctx).SpanContext().IsValid() {
			t.Fatal("non-matching request must carry trace context into handler context")
		}

		handlerSpans := findSpansByName(sr.Ended(), "dnstrace-test")
		if len(handlerSpans) != 1 {
			t.Fatalf("expected 1 handler span for non-matching request, got %d", len(handlerSpans))
		}
		assertHasDefaultAttributes(t, handlerSpans[0])
	})
}

func findSpansByName(spans []sdktrace.ReadOnlySpan, name string) []sdktrace.ReadOnlySpan {
	filtered := make([]sdktrace.ReadOnlySpan, 0, len(spans))
	for _, s := range spans {
		if s.Name() == name {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func assertHasDefaultAttributes(t *testing.T, span sdktrace.ReadOnlySpan) {
	t.Helper()
	if !hasAttribute(span.Attributes(), "dns.question.name") {
		t.Fatal("default request attribute dns.question.name is missing")
	}
	if !hasAttribute(span.Attributes(), "dns.request.type") {
		t.Fatal("default request attribute dns.request.type is missing")
	}
	if !hasAttribute(span.Attributes(), "dns.response.code") {
		t.Fatal("default response attribute dns.response.code is missing")
	}
}

func hasAttribute(attrs []attribute.KeyValue, key string) bool {
	for _, a := range attrs {
		if string(a.Key) == key {
			return true
		}
	}
	return false
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
func newTracerProvider() (*sdktrace.TracerProvider, error) {
	traceExporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter,
			sdktrace.WithBatchTimeout(time.Second)),
	)
	return tracerProvider, nil
}
