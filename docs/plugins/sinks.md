# Sink plugins

A sink receives `UsageEvent` records and persists or forwards them. All enabled sinks receive every event (fan-out).

## Interface

```go
type SinkPlugin interface {
    Name() string
    Init(config map[string]any) error
    Write(ctx context.Context, event providers.UsageEvent) error
    Close() error
}
```

## Scaffold

```sh
tokenmeter scaffold sink webhook
# creates plugins/sinks/webhook/webhook.go with stubs
```

## Built-in sinks

| Sink | Status | Config key |
|---|---|---|
| `stdout` | ✅ Available | `sinks.stdout` |
| `sqlite` | ✅ Available | `sinks.sqlite` |
| `otel` | ✅ Available | `sinks.otel` |
| `prometheus` | 🔨 v0.9 | `sinks.prometheus` |

## OTEL sink config

The OTEL sink pushes metrics via OTLP gRPC to a central collector (Grafana Cloud, Prometheus remote-write, OTel Collector, etc.).

```yaml
sinks:
  otel:
    enabled: true
    options:
      endpoint: "localhost:4317"   # gRPC endpoint (host:port)
      insecure: true               # false = TLS
      timeout_ms: 5000             # export timeout
      interval_s: 30               # push interval
```

Metrics emitted per event (attributes: `model`, `provider`, `user`):

| Metric | Type | Unit |
|---|---|---|
| `llm.tokens.input` | Counter | `{token}` |
| `llm.tokens.output` | Counter | `{token}` |
| `llm.tokens.cached` | Counter | `{token}` |
| `llm.cost.usd` | Counter | `USD` |
| `llm.latency.ms` | Histogram | `ms` |

`llm.tokens.cached` is only recorded when non-zero (avoids polluting dashboards with zero-series). `Close()` flushes with a 10 s timeout — critical for ephemeral/sidecar deployments.

## Writing a new sink

1. Run `tokenmeter scaffold sink <name>`
2. Implement `Init()` — parse config, open connections
3. Implement `Write()` — must be safe for concurrent calls; use a channel for async I/O (see the SQLite sink for the pattern)
4. Implement `Close()` — flush and release resources
5. Add blank import in `cmd/tokenmeter/main.go`

## Async write pattern

The SQLite sink uses a buffered channel so DB I/O never blocks the proxy response path:

```go
type Sink struct {
    queue chan providers.UsageEvent
    done  chan struct{}
}

func (s *Sink) Write(_ context.Context, e providers.UsageEvent) error {
    select {
    case s.queue <- e:
    default:
        slog.Warn("sink queue full — event dropped")
    }
    return nil
}

func (s *Sink) Close() error {
    close(s.queue)
    <-s.done          // wait for writeLoop to drain
    return s.db.Close()
}
```

Use this pattern for any sink where the write operation can block (HTTP, database, file I/O).

## Config shape

Each sink gets its own key under `sinks:` in `config.yaml`:

```yaml
sinks:
  mysink:
    enabled: true
    options:
      endpoint: https://my-collector.example.com
      api_key: ""    # prefer env var
```

Access in `Init()`:

```go
func (s *Sink) Init(cfg map[string]any) error {
    endpoint, _ := cfg["endpoint"].(string)
    // ...
}
```
