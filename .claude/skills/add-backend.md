# Add Backend Adapter

Scaffolds a new backend adapter under `plugins/backends/<name>/`.

Run:
```bash
tokenmeter scaffold backend <name>
```

This creates:
- `plugins/backends/<name>/<name>.go` — implements `backends.BackendAdapter`
- `plugins/backends/<name>/detect.sh`
- `plugins/backends/<name>/install.sh`
- `plugins/backends/<name>/uninstall.sh`
- `docs/backends/<name>.md` — user-facing setup guide

The adapter must:
1. Implement `Detect()` to check if the tool is installed
2. Implement `Install(proxyAddr)` to patch the tool's config or env
3. Implement `Verify(proxyAddr)` to confirm traffic flows
4. Call `backends.Register(&Adapter{})` in `init()`

See `plugins/backends/claudecode/` for a reference implementation.
