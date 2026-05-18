# Plugin overview

tokenmeter is built around four plugin surfaces. All use the same `init()` self-registration pattern — no central registry to edit, no config flags to flip.

## The four surfaces

| Interface | Package | Purpose |
|---|---|---|
| `ProviderPlugin` | `plugins/providers/` | Detect vendor, parse token counts from wire format |
| `SinkPlugin` | `plugins/sinks/` | Persist or forward `UsageEvent` |
| `MiddlewarePlugin` | `plugins/middleware/` | Transform or gate events before sinks |
| `BackendAdapter` | `plugins/backends/` | Detect + auto-configure an AI coding tool |

## Registration pattern

Every plugin registers itself in `init()`:

```go
func init() {
    providers.Register(&Plugin{})  // or sinks.Register / backends.Register
}
```

Then gets a blank import in `cmd/tokenmeter/main.go`:

```go
import _ "github.com/dvdthecoder/tokenmeter/plugins/providers/myprovider"
```

That's the entire integration contract. The core never imports plugins directly.

## Scaffold command

```sh
tokenmeter scaffold provider gemini
tokenmeter scaffold sink webhook
tokenmeter scaffold backend cursor
```

Generates the stub file with the interface pre-filled and a comment pointing to the reference implementation.

## Data flow

```
Request in
    │
    ▼
ProviderPlugin.Detect()     ← which vendor is this?
    │
    ▼
ProviderPlugin.ModifyRequest()   ← inject stream_options, etc.
    │
    ▼
    [upstream API]
    │
    ▼
ProviderPlugin.NewStreamParser() ← SSE: parse incrementally
ProviderPlugin.ParseResponse()   ← non-stream: parse body
    │
    ▼
UsageEvent{}                ← never contains prompt/response content
    │
    ▼
MiddlewarePlugin.Process()  ← redaction, cost alerts, etc.
    │
    ▼
SinkPlugin.Write()          ← fan-out to all enabled sinks
```
