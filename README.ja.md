# dnstrace - dns trace for opentelemetry

[![Go Test](https://github.com/mimuret/dnstrace/actions/workflows/go-test.yml/badge.svg)](https://github.com/mimuret/dnstrace/actions/workflows/go-test.yml)
![Coverage](docs/coverage.svg)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

DNSを使った、OpenTelemetry Trace propagationライブラリです。
EDNS0オプションを使ってトレースコンテキストを伝搬させます。Option Codeは、PowerDNSに
合わせてデフォルトで65500（0xFFDC）を使用し、`WithOptionCode`で変更可能です。

## EDNS0_TRACE仕様

EDNS0_TRACEは、EDNS0オプションを使ってtraceparent, tracestateを伝搬させるための
フォーマットです。Option CodeはPowerDNSと同じくデフォルトで65500（0xFFDC）を使用し、
`WithOptionCode`で変更できます。

先頭の固定27バイト部分（version, reserved, trace-id, span-id, trace-flags）は、
[PowerDNS](https://github.com/PowerDNS/pdns)が実装している`TRACEPARENT` EDNSオプション
（`EDNSOTTraceRecord`）のバイナリフォーマットに合わせています。`tracestate`は、この固定部分の
後ろに付加されるdnstrace独自の拡張です。

```
+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+
|                    OPTION-CODE (default 65500)                |
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

- `OPTION-CODE`: EDNS0のオプションコード（デフォルト65500 / 0xFFDC、`WithOptionCode`で変更可能）
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

### Option Codeの変更

EDNS0のOption Codeは、PowerDNSとの互換のためデフォルトで65500（0xFFDC）を使用します。
別のコードを使いたい場合は、`NewHandler` / `NewClient`に`dnstrace.WithOptionCode`を
渡します（送信側・受信側の両方で一致させる必要があります）。

```go
client := dnstrace.NewClient("sent-query", &dns.Client{}, dnstrace.WithOptionCode(0xFDE9))
```

## License

このプロジェクトは Apache-2.0 ライセンスで提供されます。詳細は [LICENSE](LICENSE) を参照してください。
