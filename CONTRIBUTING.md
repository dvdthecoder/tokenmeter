# Contributing

## Plugin architecture

There are four plugin surfaces. Each follows the same pattern:

1. Implement the interface in `plugins/<type>/<name>/<name>.go`
2. Call `<type>.Register(&Plugin{})` in an `init()` function
3. Add a blank import `_ "github.com/yourorg/tokenmeter/plugins/<type>/<name>"` in `cmd/tokenmeter/main.go`
4. Write unit tests using fixture response bodies — no network calls
5. Add yourself to `PLUGIN_REGISTRY.md`

Use `tokenmeter scaffold <type> <name>` to generate the stub.

| Type | Interface | Reference impl |
|---|---|---|
| `provider` | `providers.ProviderPlugin` | `plugins/providers/anthropic/` |
| `sink` | `sinks.SinkPlugin` | `plugins/sinks/sqlite/` |
| `middleware` | `middleware.MiddlewarePlugin` | `plugins/middleware/redaction/` |
| `backend` | `backends.BackendAdapter` | `plugins/backends/claudecode/` |

## GDPR invariant

No plugin may store, log, or forward prompt or response content. `UsageEvent` contains only metadata. PRs that violate this will not be merged.

## PR checklist

- [ ] Interface fully implemented
- [ ] Unit tests pass (`make test-race`)
- [ ] Lint passes (`make lint`)
- [ ] No prompt/response content stored
- [ ] Entry added to `PLUGIN_REGISTRY.md`
