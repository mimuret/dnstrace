// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"errors"
	"fmt"

	"github.com/miekg/dns"
)

const (
	// DefaultEDNS0TRACE is the default EDNS0 option code used to carry the trace
	// context. It matches the default TRACEPARENT option code (65500) used by
	// PowerDNS. The option code can be overridden with WithOptionCode (or the
	// *WithCode helpers below).
	DefaultEDNS0TRACE uint16 = 65500 // 0xFFDC

	// EDNS0TRACE is a deprecated alias for DefaultEDNS0TRACE.
	//
	// Deprecated: use DefaultEDNS0TRACE.
	EDNS0TRACE = DefaultEDNS0TRACE

	// traceparentBytes is the size of the fixed portion of the wire format:
	// 1 byte version, 1 byte reserved, 16 bytes trace-id, 8 bytes span-id, 1 byte trace-flags.
	// This layout matches the TRACEPARENT EDNS option used by PowerDNS (EDNSOTTraceRecord).
	traceparentBytes = 1 + 1 + 16 + 8 + 1
)

type TraceID [16]byte

func (t TraceID) String() string {
	return fmt.Sprintf("%x", t[:])
}

type SpanID [8]byte

func (s SpanID) String() string {
	return fmt.Sprintf("%x", s[:])
}

type EDNS0_TRACE struct {
	Version    byte    `json:"version"`
	Reserved   byte    `json:"reserved"`
	TraceID    TraceID `json:"trace_id"`
	SpanID     SpanID  `json:"span_id"`
	TraceFlags byte    `json:"trace_flags"`

	Tracestate []byte `json:"tracestate"`
}

func (e *EDNS0_TRACE) Traceparent() string {
	return fmt.Sprintf("%02x-%s-%s-%02x", e.Version, e.TraceID, e.SpanID, e.TraceFlags)
}

func (e *EDNS0_TRACE) Option() uint16 { return DefaultEDNS0TRACE }
func (e *EDNS0_TRACE) pack() ([]byte, error) {
	b := make([]byte, 0, traceparentBytes+len(e.Tracestate))
	b = append(b, e.Version)
	b = append(b, e.Reserved)
	b = append(b, e.TraceID[:]...)
	b = append(b, e.SpanID[:]...)
	b = append(b, e.TraceFlags)
	b = append(b, e.Tracestate...)
	return b, nil
}
func (e *EDNS0_TRACE) packLocal(code uint16) *dns.EDNS0_LOCAL {
	data, _ := e.pack()
	return &dns.EDNS0_LOCAL{
		Code: code,
		Data: data,
	}
}
func (e *EDNS0_TRACE) unpackLocal(opt *dns.EDNS0_LOCAL) error {
	if len(opt.Data) < traceparentBytes {
		return errors.New("dnstrace: bad EDNS0_TRACE length")
	}
	e.Version = opt.Data[0]
	e.Reserved = opt.Data[1]
	copy(e.TraceID[:], opt.Data[2:18])
	copy(e.SpanID[:], opt.Data[18:26])
	e.TraceFlags = opt.Data[26]
	e.Tracestate = opt.Data[27:]
	return nil
}

// GetEDNS0_TRACE returns the EDNS0_TRACE carried under the default option code.
func GetEDNS0_TRACE(m *dns.Msg) *EDNS0_TRACE {
	return GetEDNS0_TRACEWithCode(m, DefaultEDNS0TRACE)
}

// GetEDNS0_TRACEWithCode returns the EDNS0_TRACE carried under the given option code.
func GetEDNS0_TRACEWithCode(m *dns.Msg, code uint16) *EDNS0_TRACE {
	var (
		opt *dns.OPT
		ok  bool
	)
	for _, rr := range m.Extra {
		if opt, ok = rr.(*dns.OPT); ok {
			break
		}
	}
	if opt == nil {
		return nil
	}
	for _, eopt := range opt.Option {
		if eopt.Option() == code {
			if optLocal, ok := eopt.(*dns.EDNS0_LOCAL); ok {
				trace := &EDNS0_TRACE{}
				if err := trace.unpackLocal(optLocal); err != nil {
					return nil
				}
				return trace
			}
		}
	}
	return nil
}

// SetEDNS0_TRACE sets the EDNS0_TRACE under the default option code.
func SetEDNS0_TRACE(m *dns.Msg, traceOpt *EDNS0_TRACE) {
	SetEDNS0_TRACEWithCode(m, traceOpt, DefaultEDNS0TRACE)
}

// SetEDNS0_TRACEWithCode sets the EDNS0_TRACE under the given option code.
func SetEDNS0_TRACEWithCode(m *dns.Msg, traceOpt *EDNS0_TRACE, code uint16) {
	var (
		opt      *dns.OPT
		ok       bool
		localOpt = traceOpt.packLocal(code)
	)
	for _, rr := range m.Extra {
		if opt, ok = rr.(*dns.OPT); ok {
			break
		}
	}
	if opt == nil {
		o := new(dns.OPT)
		o.Hdr.Name = "."
		o.Hdr.Rrtype = dns.TypeOPT
		o.Option = []dns.EDNS0{localOpt}
		m.Extra = append(m.Extra, o)
		return
	}
	for i, eopt := range opt.Option {
		if eopt.Option() == code {
			opt.Option[i] = localOpt
			return
		}
	}
	opt.Option = append(opt.Option, localOpt)
}
