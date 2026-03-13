// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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

func startServer() (*dns.Server, net.Listener, error) {
	ln, err := nettest.NewLocalListener("tcp")
	if err != nil {
		return nil, nil, err
	}

	server := dns.Server{
		Listener: ln,
		Handler:  NewHandler("dnstrace-test", th, WithTracer(otel.Tracer("dnstrace-test"))),
	}
	go func() {
		_ = server.ActivateAndServe()
	}()
	return &server, ln, nil
}

func TestHandler(t *testing.T) {
	tracerProvider, err := newTracerProvider()
	if err != nil {
		t.Fatal(err)
	}
	otel.SetTracerProvider(tracerProvider)
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	server, ln, err := startServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = server.Shutdown()
	}()
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

func TestClient(t *testing.T) {
	tracerProvider, err := newTracerProvider()
	if err != nil {
		t.Fatal(err)
	}
	otel.SetTracerProvider(tracerProvider)
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	server, ln, err := startServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = server.Shutdown()
	}()

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
		_, _, err = client.ExchangeContext(ctx, tc.m, ln.Addr().String())
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
