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
| `prometheus` | ✅ Available | `sinks.prometheus` |

## OTEL sink config

!!! tip "Setting up a central collector"
    See [Central Collector Setup](../getting-started/collector.md) for Docker Compose, Grafana Cloud, Datadog, and other options.



The OTEL sink pushes metrics via OTLP gRPC to a central collector (Grafana Cloud, Prometheus remote-write, OTel Collector, etc.).

```yaml
sinks:
  otel:
    enabled: true
    options:
      endpoint: "localhost:4317"   # gRPC endpoint (host:port)
      insecure: true               # false = TLS (required for production)
      timeout_ms: 5000             # export timeout
      interval_s: 30               # push interval
      # Production: TLS + bearer-token auth
      tls_ca_cert: ""              # path to CA cert (from generate-certs.sh)
      bearer_token: ""             # matches TOKENMETER_COLLECTOR_TOKEN on collector
```

#### Production config (TLS + auth)

For team deployments, use the hardened collector stack in `deploy/collector/`:

```sh
cd deploy/collector
./generate-certs.sh collector.internal   # or use an IP
export TOKENMETER_COLLECTOR_TOKEN=$(openssl rand -hex 32)
export GF_SECURITY_ADMIN_PASSWORD=<your-password>
docker compose -f docker-compose.prod.yaml up -d
```

Then on each developer machine:

```yaml
sinks:
  otel:
    enabled: true
    options:
      endpoint: "collector.internal:4317"
      insecure: false
      tls_ca_cert: "/path/to/certs/ca.crt"
      bearer_token: "<TOKENMETER_COLLECTOR_TOKEN value>"
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

## Prometheus sink config

The Prometheus sink exposes a `/metrics` scrape endpoint using a private registry (does not pollute the default global registry).

```yaml
sinks:
  prometheus:
    enabled: true
    options:
      listen: "127.0.0.1:9090"   # address for the /metrics endpoint
```

Metrics exposed (labels: `model`, `provider`, `user`):

| Metric | Type | Buckets / notes |
|---|---|---|
| `llm_tokens_input_total` | Counter | — |
| `llm_tokens_output_total` | Counter | — |
| `llm_tokens_cached_total` | Counter | Only recorded when non-zero |
| `llm_cost_usd_total` | Counter | — |
| `llm_latency_ms` | Histogram | 100, 250, 500, 1000, 2500, 5000, 10000, 30000 ms |

!!! tip
    Metric names mirror the OTEL sink (`llm.tokens.input` → `llm_tokens_input_total`) so both sinks can feed the same Grafana dashboards.

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
