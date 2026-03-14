// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"encoding/hex"

	"github.com/miekg/dns"
)

func parseTraceparent(value string, opt *EDNS0_TRACE) bool {
	if len(value) != 55 {
		return false
	}
	if value[2] != '-' || value[35] != '-' || value[52] != '-' {
		return false
	}
	v, err := hex.DecodeString(value[:2])
	if err != nil || len(v) != 1 {
		return false
	}
	opt.Version = v[0]
	v, err = hex.DecodeString(value[3:35])
	if err != nil || len(v) != 16 {
		return false
	}
	copy(opt.TraceID[:], v)
	v, err = hex.DecodeString(value[36:52])
	if err != nil || len(v) != 8 {
		return false
	}
	copy(opt.SpanID[:], v)
	v, err = hex.DecodeString(value[53:55])
	if err != nil || len(v) != 1 {
		return false
	}
	opt.TraceFlags = v[0]
	return true
}

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
		if !parseTraceparent(value, opt) {
			return
		}
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
