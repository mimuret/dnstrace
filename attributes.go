package dnstrace

import (
	"net"
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

func DNSQuestionType(val uint16) attribute.KeyValue {
	qtype, ok := dns.TypeToString[val]
	if !ok {
		qtype = "OTHER"
	}
	return attribute.String("dns.request.type", qtype)
}

func DNSResponseCode(val int) attribute.KeyValue {
	rcode, ok := dns.RcodeToString[val]
	if !ok {
		rcode = "OTHER"
	}
	return attribute.String("dns.response.code", rcode)
}

func SetRequestAttributes(span trace.Span, m *dns.Msg, serverAddr string, clientAddr string) {
	if len(m.Question) > 0 {
		qname := m.Question[0].Name
		span.SetAttributes(
			semconv.DNSQuestionName(qname),
			DNSQuestionType(m.Question[0].Qtype),
		)
	}
	serverHost, serverPort, err := SplitHostPort(serverAddr)
	if err == nil {
		span.SetAttributes(
			semconv.ServerAddress(serverHost),
			semconv.ServerPort(serverPort),
		)
	}
	clientHost, clientPort, err := SplitHostPort(clientAddr)
	if err == nil {
		span.SetAttributes(
			semconv.ClientAddress(clientHost),
			semconv.ClientPort(clientPort),
		)
	}
}

func SetResponseAttributes(span trace.Span, r *dns.Msg, rtt time.Duration, err error) {
	if err != nil {
		return
	}
	if r == nil {
		return
	}
	span.SetAttributes(
		DNSResponseCode(r.Rcode),
	)
}

func SplitHostPort(addr string) (host string, port int, err error) {
	var portStr string
	host, portStr, err = net.SplitHostPort(addr)
	if err != nil {
		return
	}
	port, err = net.LookupPort("udp", portStr)
	if err != nil {
		return
	}
	return host, port, nil
}
