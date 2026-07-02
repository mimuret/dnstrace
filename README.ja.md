# dnstrace - dns trace for opentelemetry

[![Go Test](https://github.com/mimuret/dnstrace/actions/workflows/go-test.yml/badge.svg)](https://github.com/mimuret/dnstrace/actions/workflows/go-test.yml)
![Coverage](docs/coverage.svg)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

DNSを使った、OpenTelemetry Trace propagationライブラリです。
EDNS0のPrivate用のOption Code 0xFDE9 を使ってトレースコンテキストを伝搬させます。

## EDNS0_TRACE仕様

EDNS0_TRACEは、EDNS0のPrivate用のOption Code 0xFDE9 を使ってtraceparent, tracestateを伝搬させるためのフォーマットです。

先頭の固定27バイト部分（version, reserved, trace-id, span-id, trace-flags）は、
[PowerDNS](https://github.com/PowerDNS/pdns)が実装している`TRACEPARENT` EDNSオプション
（`EDNSOTTraceRecord`）のバイナリフォーマットに合わせています。`tracestate`は、この固定部分の
後ろに付加されるdnstrace独自の拡張です。

```
+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+
|                       OPTION-CODE (0xFDE9)                    |
+---+---+---+---+---+---+---+---|---+---+---+---+---+---+---+---+
|                       OPTION-LENGTH (2byte)                   |
+---+---+---+---+---+---+---+---|---+---+---+---+---+---+---+---+
|          VERSION(1byte)       |        RESERVED(1byte)        |
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
|       TRACE-FLAGS(1byte)      |        TRACE-STATE...         |
+---+---+---+---+---+---+---+---|---+---+---+---+---+---+---+---+--+
```

- `OPTION-CODE`: EDNS0のオプションコード
- `OPTION-LENGTH`: EDNS0オプションの長さ
- `VERSION`: traceparentのversion
- `RESERVED`: 予約バイト（0固定）。PowerDNSとの互換のためのフィールド
- `TRACE-ID`: traceparentのtrace-id
- `SPAN-ID`: traceparentのspan-id
- `TRACE-FLAGS`: traceparentのtrace-flags
- `TRACE-STATE`: tracestateのASCII文字列（オプション）

traceparent, tracestateのフォーマットは[Trace Context](https://www.w3.org/TR/trace-context/)に準拠します。


## Usage

詳細は [dnstrace_test.go](dnstrace_test.go)を参照

### サーバ側

- `dns.ServerのHandler`には、既存のhandlerを渡した`dnstrace.NewHandler`を指定します。
- handlerは、`ServeDNS`では`context.Context`が渡されないため、`ServeDNSWithContext`を実装する必要があります。
- DNSクエリを受け取ると、EDNS0_TRACEが存在すれば、propagatorを使ってspanが追加され、なければ新規にspanが追加されます。
- `ServeDNSWithContext`の`context.Context`にトレースコンテキストが入っているため、それを使ってトレースできます。

### クライアント側

- `dns.Client`を、`dnstrace.NewClient`でラップします。
- `ExchangeContext`または、`ExchangeWithConnContext`を使って、トレースコンテキストが入っている`context.Context`を渡します。
  - `context.Context`に、トレースコンテキストが入っていれば、EDNS0_TRACEが追加されます。すでにEDNS0_TRACEが設定されている場合は、上書きされます。

## License

このプロジェクトは Apache-2.0 ライセンスで提供されます。詳細は [LICENSE](LICENSE) を参照してください。
