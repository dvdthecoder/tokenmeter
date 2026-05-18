# Add Provider Plugin

Scaffolds a new LLM provider plugin under `plugins/providers/<name>/`.

Run:
```bash
tokenmeter scaffold provider <name>
```

This creates:
- `plugins/providers/<name>/<name>.go` — implements `providers.ProviderPlugin`
- `plugins/providers/<name>/<name>_test.go` — unit tests with fixture response bodies

The plugin must:
1. Implement `Detect()` to match requests by Host or URL path
2. Implement `ParseUsage()` for non-streaming responses
3. Implement `ParseStreamUsage()` for SSE tail parsing
4. Call `providers.Register(&Plugin{})` in `init()`
5. Add a blank import in `cmd/tokenmeter/main.go`

See `plugins/providers/anthropic/` for a reference implementation.
