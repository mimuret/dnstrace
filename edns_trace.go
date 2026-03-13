// SPDX-License-Identifier: Apache-2.0

package dnstrace

import (
	"errors"
	"fmt"

	"github.com/miekg/dns"
)

const (
	EDNS0TRACE       = 0xFDE9
	traceparentBytes = 1 + 16 + 8 + 1
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
	TraceID    TraceID `json:"trace_id"`
	SpanID     SpanID  `json:"span_id"`
	TraceFlags byte    `json:"trace_flags"`

	Tracestate []byte `json:"tracestate"`
}

func (e *EDNS0_TRACE) Traceparent() string {
	return fmt.Sprintf("%02x-%s-%s-%02x", e.Version, e.TraceID, e.SpanID, e.TraceFlags)
}

func (e *EDNS0_TRACE) Option() uint16 { return EDNS0TRACE }
func (e *EDNS0_TRACE) pack() ([]byte, error) {
	b := make([]byte, 0, traceparentBytes+len(e.Tracestate))
	b = append(b, e.Version)
	b = append(b, e.TraceFlags)
	b = append(b, e.TraceID[:]...)
	b = append(b, e.SpanID[:]...)
	b = append(b, e.Tracestate...)
	return b, nil
}
func (e *EDNS0_TRACE) packLocal() *dns.EDNS0_LOCAL {
	data, _ := e.pack()
	return &dns.EDNS0_LOCAL{
		Code: EDNS0TRACE,
		Data: data,
	}
}
func (e *EDNS0_TRACE) unpackLocal(opt *dns.EDNS0_LOCAL) error {
	if len(opt.Data) < traceparentBytes {
		return errors.New("dnstrace: bad EDNS0_TRACE length")
	}
	e.Version = opt.Data[0]
	e.TraceFlags = opt.Data[1]
	copy(e.TraceID[:], opt.Data[2:18])
	copy(e.SpanID[:], opt.Data[18:26])
	e.Tracestate = opt.Data[26:]
	return nil
}

func GetEDNS0_TRACE(m *dns.Msg) *EDNS0_TRACE {
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
		if eopt.Option() == EDNS0TRACE {
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

func SetEDNS0_TRACE(m *dns.Msg, traceOpt *EDNS0_TRACE) {
	var (
		opt      *dns.OPT
		ok       bool
		localOpt = traceOpt.packLocal()
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
		if eopt.Option() == EDNS0TRACE {
			opt.Option[i] = localOpt
			return
		}
	}
	opt.Option = append(opt.Option, localOpt)
}
