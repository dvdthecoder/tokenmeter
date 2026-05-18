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
| `otel` | 🔨 v0.5 | `sinks.otel` |
| `prometheus` | 🔨 v0.5 | `sinks.prometheus` |

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
