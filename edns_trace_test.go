// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"testing"

	"github.com/miekg/dns"
)

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
