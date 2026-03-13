# dnstrace - dns trace for opentelemetry

[![Go Test](https://github.com/mimuret/dnstrace/actions/workflows/go-test.yml/badge.svg)](https://github.com/mimuret/dnstrace/actions/workflows/go-test.yml)
![Coverage](docs/coverage.svg)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

dnstrace is a library for OpenTelemetry trace propagation over DNS.
It propagates trace context using EDNS0 private-use option code 0xFDE9.

## EDNS0_TRACE specification

EDNS0_TRACE is a format for propagating `traceparent` and `tracestate` using EDNS0 private-use option code 0xFDE9.

```
+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+
|                       OPTION-CODE (0xFDE9)                    |
+---+---+---+---+---+---+---+---|---+---+---+---+---+---+---+---+
|                       OPTION-LENGTH (2byte)                   |
+---+---+---+---+---+---+---+---|---+---+---+---+---+---+---+---+
|          VERSION(1byte)       |       TRACE-FLAGS(1byte)      |
+---+---+---+---+---+---+---+---|---+---+---+---+---+---+---+---+
|                                                               |
|                                                               |
|                                                               |
|                                                               |
|                       TRACE-ID(16 byte)                       |
|                                                               |
|                                                               |
|                                                               |
+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+
|                                                               |
|                                                               |
|                       SPAN-ID(8 byte)                         |
|                                                               |
+---+---+---+---+---+---+---+---|---+---+---+---+---+---+---+---+
|                       TRACE-STATE...                          |
+---+---+---+---+---+---+---+---|---+---+---+---+---+---+---+---+--+
```

- <OPTION-CODE>: EDNS0 option code
- <VERSION>: traceparent version
- <TRACE-ID>: traceparent trace-id
- <SPAN-ID>: traceparent span-id
- <TRACE-FLAGS>: traceparent trace-flags
- <TRACE-STATE>: ASCII string for tracestate

`traceparent` and `tracestate` follow the [Trace Context](https://www.w3.org/TR/trace-context/) specification.


## Usage

See [dnstrace_test.go](dnstrace_test.go) for details.

### Server side

- Set `dns.Server` handler to `dnstrace.NewHandler` wrapping your existing handler.
- Since `context.Context` is not passed to `ServeDNS`, your handler needs to implement `ServeDNSWithContext`.
- When a DNS query is received, if EDNS0_TRACE exists, a span is added using the propagator; otherwise, a new span is created.
- The trace context is available in the `context.Context` passed to `ServeDNSWithContext`.

### Client side

- Wrap `dns.Client` with `dnstrace.NewClient`.
- Pass a `context.Context` containing trace context via `ExchangeContext` or `ExchangeWithConnContext`.
- If the `context.Context` contains trace context, EDNS0_TRACE is added. If EDNS0_TRACE already exists, it is overwritten.

## License

This project is licensed under Apache-2.0. See [LICENSE](LICENSE).

