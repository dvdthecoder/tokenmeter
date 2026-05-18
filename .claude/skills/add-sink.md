# Add Sink Plugin

Scaffolds a new sink plugin under `plugins/sinks/<name>/`.

Run:
```bash
tokenmeter scaffold sink <name>
```

This creates:
- `plugins/sinks/<name>/<name>.go` — implements `sinks.SinkPlugin`
- `plugins/sinks/<name>/<name>_test.go`

The plugin must:
1. Implement `Write(ctx, event)` — safe for concurrent calls
2. Implement `Init(config)` to read its config block from `config.yaml`
3. Call `sinks.Register(&Sink{})` in `init()`
4. Add a blank import in `cmd/tokenmeter/main.go`

See `plugins/sinks/sqlite/` for a reference implementation.
