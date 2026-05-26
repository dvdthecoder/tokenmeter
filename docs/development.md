# Development

This page covers how to build, test, and extend tokenmeter locally.

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.23+ | Build + test the proxy |
| Node.js | 20+ | Build the VS Code extension |
| Docker | any | Optional — collector stack only (`make dev-up`) |

## Build

```sh
make build          # → bin/tokenmeter
make release        # cross-compile all platforms → dist/
```

## Running the tests

```sh
make test           # go test ./...
make test-race      # go test -race ./...  ← required before every PR
make lint           # go vet + go build (no test binary needed)
```

All three must pass clean before opening a PR. `make test-race` catches data races in the streaming and sink fan-out paths — never skip it.

### Run a single package

```sh
go test ./plugins/providers/anthropic/... -v
go test ./internal/proxy/... -v -run TestStreamParser
```

### Run a single test function

```sh
go test ./plugins/providers/gemini/... -run TestStreamParserFinalChunkWins -v
```

## Smoke test

`make smoke` is the CI gate — it builds the binary, starts the proxy against `config.dev.yaml`, hits `/health`, then stops. No live API keys, no Docker needed.

```sh
make smoke
```

Expected output:

```
--- smoke: starting proxy ---
Proxy started (PID 12345)
  Listening: 127.0.0.1:4191
  Logs:      /tmp/tokenmeter-dev.log
  DB:        /tmp/tokenmeter-dev.db
--- smoke: checking proxy health ---
  health: OK
--- smoke: done ---
Proxy stopped (PID 12345)
```

---

## Test architecture

All tests are **fixture-based** — they replay recorded SSE payloads and assert token counts. No network calls, no real API keys.

### Provider tests

Each provider has a `streamFixture` (slice of raw SSE lines) and a `nonStreamFixture` (raw JSON body). The pattern:

```go
var streamFixture = []string{
    // real Anthropic SSE events, copied from API responses
    `{"type":"message_start","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":142,...}}}`,
    `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":75}}`,
    `{"type":"message_stop"}`,
}

func TestStreamParser(t *testing.T) {
    p := &Plugin{}
    sp := p.NewStreamParser()
    for _, line := range streamFixture {
        _ = sp.ConsumeEvent([]byte(line))
    }
    _, _, _, input, output, cached, _ := sp.Result()
    // assert counts ...
}
```

See `plugins/providers/anthropic/anthropic_test.go` for the full reference implementation.

### Sink tests

SQLite tests use an in-memory database — no temp file cleanup needed:

```go
db, _ := storage.Open(":memory:")
defer db.Close()
```

The `openMemDB(t *testing.T)` helper in `plugins/sinks/sqlite/sqlite_test.go` registers a `t.Cleanup` for you.

### Backend tests

Backend adapter tests verify `Detect()`, `Install()`, and `Verify()` against temp files and environment variables. See `plugins/backends/claudecode/claudecode_test.go`.

---

## VS Code extension

The extension lives in `extensions/vscode/`. It is a standard TypeScript + esbuild project.

```sh
cd extensions/vscode
npm install          # install dev deps (esbuild, @types/vscode, typescript)
npm run compile      # type-check only (no output files)
npm run build        # bundle → out/extension.js (minified, ~13 KB)
npm run watch        # rebuild on save — use alongside F5 dev host
```

### Run the extension in a dev host

1. Open `extensions/vscode/` as a workspace in VS Code
2. Press **F5** — VS Code opens an Extension Development Host window with the extension loaded
3. The status bar item and all commands are live; editing `src/` and saving rebuilds automatically with `npm run watch`

---

## Adding a new plugin

1. Generate the stub: `tokenmeter scaffold <type> <name>`
   - Types: `provider`, `sink`, `middleware`, `backend`
2. Implement the interface — use the nearest sibling as reference
3. Add a blank import in `cmd/tokenmeter/main.go`:
   ```go
   _ "github.com/dvdthecoder/tokenmeter/plugins/<type>/<name>"
   ```
4. Write fixture tests — no network calls, no real keys
5. Run `make test-race` and `make lint`
6. Add an entry to `PLUGIN_REGISTRY.md`
7. Open a PR

### GDPR invariant

No plugin may store, log, or forward prompt or response content. `UsageEvent` contains only metadata fields (token counts, cost, latency, model name). PRs that violate this will not be merged.

### PR checklist

- [ ] Interface fully implemented
- [ ] `make test-race` passes
- [ ] `make lint` passes
- [ ] No prompt/response content stored or logged
- [ ] Entry added to `PLUGIN_REGISTRY.md`
