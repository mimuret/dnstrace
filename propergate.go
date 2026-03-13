// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"encoding/hex"

	"github.com/miekg/dns"
)

type DNSMsgCarrier struct {
	msg *dns.Msg
}

func NewDNSMsgCarrier(m *dns.Msg) *DNSMsgCarrier {
	return &DNSMsgCarrier{msg: m}
}

func (c *DNSMsgCarrier) Get(key string) string {
	opt := GetEDNS0_TRACE(c.msg)
	if opt == nil {
		return ""
	}
	if key == "traceparent" {
		return opt.Traceparent()
	}
	if key == "tracestate" {
		return string(opt.Tracestate)
	}
	return ""
}

func (c *DNSMsgCarrier) Set(key string, value string) {
	opt := GetEDNS0_TRACE(c.msg)
	if opt == nil {
		opt = &EDNS0_TRACE{}
	}
	if key == "traceparent" {
		// <version>-<trace-id>-<parent-id>-<trace-flags>
		// <version> 2HEXDIGLC
		// <trace-id> 32HEXDIGLC
		// <parent-id> 16HEXDIGLC
		// <trace-flags> 2HEXDIGLC
		if len(value) < 55 {
			return
		}
		v, _ := hex.DecodeString(value[:2])
		opt.Version = v[0]
		v, _ = hex.DecodeString(value[3:35])
		copy(opt.TraceID[:], v)
		v, _ = hex.DecodeString(value[36:52])
		copy(opt.SpanID[:], v)
		v, _ = hex.DecodeString(value[53:55])
		opt.TraceFlags = v[0]
	}
	if key == "tracestate" {
		opt.Tracestate = []byte(value)
	}
	SetEDNS0_TRACE(c.msg, opt)
}

func (c *DNSMsgCarrier) Keys() []string {
	if GetEDNS0_TRACE(c.msg) != nil {
		return []string{"traceparent", "tracestate"}
	}
	return nil
}
