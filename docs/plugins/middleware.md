# Middleware plugins

Middleware transforms or gates `UsageEvent` records after the proxy captures them but before any sink receives them. All enabled middleware runs in registration order as a chain; returning an error from `Process()` drops the event (useful for rate limiting or blocked-model enforcement).

## Interface

```go
type MiddlewarePlugin interface {
    Name() string
    Init(config map[string]any) error
    Process(ctx context.Context, event *providers.UsageEvent) error
}
```

## Scaffold

```sh
tokenmeter scaffold middleware costalert
# creates plugins/middleware/costalert/costalert.go with stubs
```

## Built-in middleware

| Name | Status | Config key |
|---|---|---|
| `redaction` | ✅ Available | `middleware[].name: redaction` |
| `costalert` | ✅ Available | `middleware[].name: costalert` |

## Redaction middleware

Replaces PII in configurable event fields using Go regex patterns. Runs before every sink — useful for stripping emails or usernames before pushing to a shared OTEL collector.

```yaml
middleware:
  - name: redaction
    options:
      enabled: true
      patterns:
        - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b'  # email addresses
        - '\b\d{3}-\d{2}-\d{4}\b'                                   # SSNs
        - 'acme-corp'                                                 # literal string
      fields:
        - username      # default
        - service_id    # default
        - session_id
        - client_name
```

Each matching substring is replaced with `[REDACTED]`. Patterns are applied to each configured field independently.

| Option | Default | Description |
|---|---|---|
| `enabled` | `true` | Toggle the middleware on/off without removing the config block |
| `patterns` | `[]` | Go regex patterns (`regexp.MustCompile` syntax) |
| `fields` | `["username", "service_id"]` | Fields to scan; valid values: `username`, `service_id`, `session_id`, `client_name` |

!!! note
    Redaction runs at the middleware layer — after token counts and cost are captured, but before any persistence. It cannot affect billing accuracy.

## Costalert middleware

Fires a `slog.Warn` and an optional webhook POST when a single request exceeds a USD cost threshold. The event is never dropped — alerts are informational only.

```yaml
middleware:
  - name: costalert
    options:
      enabled: true
      threshold_usd: 0.10         # alert when event.CostUSD >= this value
      webhook_url: ""             # optional; POST alert payload here
      timeout_ms: 3000            # webhook request timeout
```

The webhook payload extends `UsageEvent` with two fields:

```json
{
  "alert": "cost_threshold_exceeded",
  "threshold_usd": 0.10,
  "request_id": "...",
  "model": "claude-opus-4-1",
  "cost_usd": 0.52,
  ...
}
```

!!! tip
    Combine with the [webhook sink](sinks.md#webhook-sink-config) to get both per-event delivery and threshold alerts, or use costalert alone for noisy-event suppression.

## Writing custom middleware

1. Run `tokenmeter scaffold middleware <name>`
2. Implement `Init()` — parse config options
3. Implement `Process()` — mutate `*event` in place; return an error to drop the event entirely
4. Add a blank import in `cmd/tokenmeter/main.go`

Example — block events for a specific model:

```go
func (m *Plugin) Process(_ context.Context, e *providers.UsageEvent) error {
    if e.Model == m.blockedModel {
        return fmt.Errorf("model %q is blocked by policy", e.Model)
    }
    return nil
}
```

## Config shape

Middleware is a list (order matters — chain runs top to bottom):

```yaml
middleware:
  - name: redaction
    options:
      patterns: ['\b\d{3}-\d{2}-\d{4}\b']
  - name: costalert
    options:
      threshold_usd: 0.10
      webhook_url: https://hooks.example.com/alert
```
