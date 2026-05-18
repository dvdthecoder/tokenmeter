# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project is

`tokenmeter` is a thin, GDPR-compliant Go interceptor that sits between AI coding tools (Claude Code, Codex CLI, OpenCode, VS Code extensions) and LLM APIs (Anthropic, OpenAI). It extracts token usage metadata from every request/response — without ever storing prompt or response content — and emits it via OpenTelemetry or to a local SQLite database for cost and usage analysis.

Deployment targets: **sidecar** (loopback-only, per-agent) and **shared** (multi-agent). Runs as a system daemon (launchd / systemd / Windows Service).

## Commands

```bash
make build          # compile to bin/tokenmeter
make test           # go test ./...
make test-race      # go test -race ./...
make lint           # go vet + go build
make release        # cross-compile all platform binaries to dist/
make smoke          # integration smoke test: build → start → /health → stop

go test ./plugins/providers/... -run TestAnthropicParse   # single test
go test ./internal/proxy/... -v                            # package with verbose output
```

Local dev environment (edge proxy + collector stack):
```bash
make dev-up              # start everything: OTel Collector + Prometheus + Grafana + edge proxy
make dev-down            # stop everything

make collector-up        # Docker: OTel Collector + Prometheus + Grafana only
make collector-down      # stop collector containers
make collector-logs      # tail OTel Collector output (see metrics arrive in real time)
make collector-open      # open Grafana at localhost:3000 (admin / tokenmeter)

make dev-proxy           # build + start edge proxy in background (uses config.dev.yaml)
make dev-proxy-stop      # stop proxy + clear stale process on :4191
make dev-logs            # tail /tmp/tokenmeter-dev.log
make dev-query           # tokenmeter query --last 1h against dev SQLite
make dev-status          # show proxy PID + Docker container status
```

Runtime:
```bash
./bin/tokenmeter start                           # foreground
./bin/tokenmeter daemon                          # background daemon
./bin/tokenmeter install                         # daemon + auto-configure detected tools
./bin/tokenmeter install --backend claudecode    # specific backend only
./bin/tokenmeter verify                          # health check + routing confirmation
./bin/tokenmeter status
./bin/tokenmeter query --last 24h --format table
./bin/tokenmeter purge --before 2024-01-01
./bin/tokenmeter export --format csv
./bin/tokenmeter scaffold provider <name>        # generate plugin stub
```

## Architecture

### Plugin surfaces (all use init() self-registration)

| Package | Interface | Purpose |
|---|---|---|
| `plugins/providers/` | `ProviderPlugin` | Detect vendor, parse token counts from wire format |
| `plugins/sinks/` | `SinkPlugin` | Persist or forward `UsageEvent` (fan-out to all enabled sinks) |
| `plugins/middleware/` | `MiddlewarePlugin` | Transform/gate events before sinks (redaction, cost alerts) |
| `plugins/backends/` | `BackendAdapter` | Detect + configure AI coding tools to route through tokenmeter |

All four interfaces follow the same pattern: implement the interface, call `Register()` in `init()`, add a blank import in `cmd/tokenmeter/main.go`. The `tokenmeter scaffold <type> <name>` command generates the stub.

### Core data flow

```
AI Tool → tokenmeter (SSE stream forwarded in real time)
                ↓ tail parsed for usage on stream end
           UsageEvent{}           ← never contains prompt/response content
                ↓
         middleware chain         ← redaction, cost alert, etc.
                ↓
         sink fan-out             ← sqlite, otel, prometheus (all enabled sinks)
```

### Streaming (critical path)

SSE responses are forwarded chunk-by-chunk to the caller. The interceptor buffers only the last ~2 KB of the stream to extract the usage event from the terminal SSE event (`message_stop` for Anthropic, the final `data: [DONE]` chunk for OpenAI). See `internal/proxy/stream.go`.

### GDPR / privacy invariants

- `UsageEvent` contains **no prompt or response content** — metadata only.
- `service_id` is SHA-256 hashed before reaching the storage layer when `hash_service_id: true` (default).
- The tokenmeter binary makes zero outbound connections except to the upstream model API and the configured OTEL collector.
- Retention auto-purge runs at daemon startup.

### Built-in providers

- `plugins/providers/anthropic/` — parses Anthropic SSE (`message_delta`, `message_stop`) and REST responses
- `plugins/providers/openai/` — parses OpenAI streaming and non-streaming `usage` fields; also covers OpenAI-compatible APIs (OpenCode, LiteLLM, Ollama)

### Built-in sinks

- `plugins/sinks/sqlite/` — local SQLite, privacy-first schema (metrics table + separate erasable service registry)
- `plugins/sinks/otel/` — OTLP gRPC exporter; metrics: `llm.tokens.input/output/cached`, `llm.cost.usd`, `llm.latency.ms`
- `plugins/sinks/prometheus/` — `/metrics` scrape endpoint

### Built-in backends (AI tool integrations)

- `plugins/backends/claudecode/` — patches `ANTHROPIC_BASE_URL`; also installs `.claude/skills/` for in-tool commands
- `plugins/backends/codex/` — patches `OPENAI_BASE_URL` in shell profile
- `plugins/backends/opencode/` — patches `~/.config/opencode/config.json`
- `plugins/backends/vscode/` — patches Continue.dev and Cline extension settings

## Claude Code skills (`.claude/skills/`)

| Skill | Trigger | Purpose |
|---|---|---|
| `install-proxy` | `/install-proxy` | Download binary, run `tokenmeter install`, verify routing |
| `proxy-status` | `/proxy-status` | Daemon health + recent events |
| `proxy-purge` | `/proxy-purge` | GDPR erasure |
| `proxy-report` | `/proxy-report` | Cost + token summary |
| `add-provider` | `/add-provider` | Scaffold new provider plugin |
| `add-sink` | `/add-sink` | Scaffold new sink plugin |
| `add-backend` | `/add-backend` | Scaffold new backend adapter |

## Key config

`config.example.yaml` is the canonical reference. Copy to `config.yaml` to use. The `TOKENMETER_ENCRYPTION_KEY` env var sets SQLite encryption key when `encrypt_at_rest: true`. `ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL` are what downstream tools read — both are set to `http://127.0.0.1:4191` by `tokenmeter install`.

## Adding a new plugin (contributor workflow)

1. Run `tokenmeter scaffold <type> <name>` to generate the stub.
2. Implement the interface — see same-type sibling for reference.
3. Add blank import in `cmd/tokenmeter/main.go`.
4. Write unit tests with fixture response bodies (no network calls in tests).
5. Open PR using the plugin contribution issue template.
