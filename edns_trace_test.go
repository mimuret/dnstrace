// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"testing"

	"github.com/miekg/dns"
)

// TestEDNS0_TRACE_WireLayout verifies the on-wire byte layout matches the
// PowerDNS TRACEPARENT EDNS option (EDNSOTTraceRecord):
// 1 byte version, 1 byte reserved, 16 bytes trace-id, 8 bytes span-id, 1 byte trace-flags,
// followed by an optional tracestate.
func TestEDNS0_TRACE_WireLayout(t *testing.T) {
	in := &EDNS0_TRACE{
		Version:    0x00,
		Reserved:   0x00,
		TraceID:    [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
		TraceFlags: 0x01,
		Tracestate: []byte("key1=value1"),
	}

	data, err := in.pack()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 27+len(in.Tracestate) {
		t.Fatalf("unexpected packed length: %d", len(data))
	}
	if data[0] != in.Version {
		t.Fatalf("version at offset 0: got %#x", data[0])
	}
	if data[1] != in.Reserved {
		t.Fatalf("reserved at offset 1: got %#x", data[1])
	}
	for i, b := range in.TraceID {
		if data[2+i] != b {
			t.Fatalf("trace-id byte %d mismatch", i)
		}
	}
	for i, b := range in.SpanID {
		if data[18+i] != b {
			t.Fatalf("span-id byte %d mismatch", i)
		}
	}
	if data[26] != in.TraceFlags {
		t.Fatalf("trace-flags at offset 26: got %#x", data[26])
	}
	if string(data[27:]) != string(in.Tracestate) {
		t.Fatalf("tracestate after offset 27: got %q", string(data[27:]))
	}

	var out EDNS0_TRACE
	if err := out.unpackLocal(&dns.EDNS0_LOCAL{Code: EDNS0TRACE, Data: data}); err != nil {
		t.Fatal(err)
	}
	if out.Traceparent() != in.Traceparent() {
		t.Fatalf("traceparent roundtrip mismatch: %s != %s", out.Traceparent(), in.Traceparent())
	}
	if string(out.Tracestate) != string(in.Tracestate) {
		t.Fatalf("tracestate roundtrip mismatch: %q != %q", out.Tracestate, in.Tracestate)
	}
}

func TestGetEDNS0_TRACE_MalformedLocalData(t *testing.T) {
	m := new(dns.Msg)
	o := &dns.OPT{}
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	o.Option = []dns.EDNS0{
		&dns.EDNS0_LOCAL{
			Code: EDNS0TRACE,
			Data: []byte{0x00, 0x01},
		},
	}
	m.Extra = append(m.Extra, o)

	if got := GetEDNS0_TRACE(m); got != nil {
		t.Fatal("malformed EDNS0 data must be ignored")
	}
}

func TestDNSMsgCarrierSetTraceparent_InvalidFormat(t *testing.T) {
	testCases := []struct {
		name  string
		value string
	}{
		{name: "too short", value: "00-0123"},
		{name: "invalid delimiter", value: "00_0123456789abcdef0123456789abcdef-0123456789abcdef-01"},
		{name: "invalid trace-id hex", value: "00-0123456789abcdef0123456789abcdeg-0123456789abcdef-01"},
		{name: "invalid span-id hex", value: "00-0123456789abcdef0123456789abcdef-0123456789abcdeg-01"},
		{name: "invalid trace-flags hex", value: "00-0123456789abcdef0123456789abcdef-0123456789abcdef-0g"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := new(dns.Msg)
			c := NewDNSMsgCarrier(m)

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Set(traceparent) must not panic: %v", r)
					}
				}()
				c.Set("traceparent", tc.value)
			}()

			if got := GetEDNS0_TRACE(m); got != nil {
				t.Fatalf("invalid traceparent must be ignored: %s", tc.value)
			}
		})
	}
}

func TestDNSMsgCarrierSetTraceparent_InvalidKeepsExistingValue(t *testing.T) {
	m := new(dns.Msg)
	SetEDNS0_TRACE(m, &EDNS0_TRACE{
		Version:    0x00,
		TraceID:    [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanID:     [8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
		TraceFlags: 0x01,
	})
	before := GetEDNS0_TRACE(m)
	if before == nil {
		t.Fatal("initial trace must exist")
	}

	c := NewDNSMsgCarrier(m)
	c.Set("traceparent", "00-0123456789abcdef0123456789abcdeg-0123456789abcdef-01")

	after := GetEDNS0_TRACE(m)
	if after == nil {
		t.Fatal("trace must remain after invalid update")
	}
	if after.Traceparent() != before.Traceparent() {
		t.Fatalf("traceparent changed unexpectedly: before=%s after=%s", before.Traceparent(), after.Traceparent())
	}
}
