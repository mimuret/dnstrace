# dnstrace - dns trace for opentelemetry

[![Go Test](https://github.com/mimuret/dnstrace/actions/workflows/go-test.yml/badge.svg)](https://github.com/mimuret/dnstrace/actions/workflows/go-test.yml)
![Coverage](docs/coverage.svg)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

DNSを使った、OpenTelemetry Trace propagationライブラリです。
EDNS0のPrivate用のOption Code 0xFDE9 を使ってトレースコンテキストを伝搬させます。

## EDNS0_TRACE仕様

EDNS0_TRACEは、EDNS0のPrivate用のOption Code 0xFDE9 を使ってtraceparent, tracestateを伝搬させるためのフォーマットです。

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

- <OPTION-CODE>: EDNS0のオプションコード
- <VERSION>: traceparentのversion
- <TRACE-ID>: traceparentのtrace-id
- <SPAN-ID>: traceparentのspan-id
- <TRACE-FLAGS>: traceparentのtrace-flags
- <TRACE-STATE>: tracestateのASCII文字列

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
