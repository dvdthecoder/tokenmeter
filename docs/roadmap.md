# Roadmap

tokenmeter is built in focused iterations. Each ships a working, tested slice — no half-finished features.

## Status

| Iteration | Version | Theme | Status |
|---|---|---|---|
| 1 | — | Traffic flows, tokens captured | ✅ Done |
| 2 | — | SQLite persistence + query CLI | ✅ Done |
| 3 | — | Daemon + install | ✅ Done |
| 4 | v0.4 | Backend integrations | ✅ Done |
| 5 | v0.5 | OTEL + Prometheus + Gemini | Planned |
| 6 | v0.6 | GDPR tooling + redaction | Planned |
| 7 | v0.7 | Plugin scaffold + contribution tooling | Planned |
| 8 | v0.8 | VS Code extension | Planned |

---

## ✅ Iteration 1 — Traffic flows, tokens captured

- Reverse proxy engine (`net/http/httputil.ReverseProxy`)
- Anthropic provider: SSE streaming + REST, all cache tiers (`cache_read`, `cache_creation`, ephemeral 5m/1h)
- OpenAI provider: streaming (with `stream_options` injection) + REST, vLLM support
- Full `UsageEvent` schema: client attribution, service tier, inference geo, cache write tokens
- Stdout sink for dev mode
- Client detection: `claude-cli/2.1.142` → `claude-code-cli@2.1.142`
- Hardening: upstream timeouts, stream buffer cap, graceful SIGTERM shutdown

## ✅ Iteration 2 — SQLite persistence + query CLI

- Local SQLite sink with async write queue (non-blocking proxy path)
- Auto-purge on startup (configurable retention)
- `tokenmeter query` — table / JSON / CSV, filters: model, user, time window, limit
- `tokenmeter purge` — GDPR date-range deletion
- `tokenmeter export` — full dump in JSON or CSV
- In-memory tests (no disk I/O in CI)

## ✅ Iteration 3 — Daemon + install

- `tokenmeter daemon` — re-exec with `Setsid`, PID file, logs to platform path
- `tokenmeter stop` — SIGTERM + 10 s drain + force kill
- `tokenmeter status` — running/stopped + last 5 events
- `tokenmeter install` — writes config, installs launchd plist (macOS) or systemd user unit (Linux), patches shell profile
- `tokenmeter uninstall` — clean reversal of all install actions
- Idempotent shell patching for zsh, bash, fish

## ✅ Iteration 4 — Backend integrations (v0.4)

Each AI tool gets an adapter that auto-detects and configures itself:

- **Claude Code** — skill files installed to `~/.claude/skills/` (`/proxy-status`, `/proxy-report`, `/proxy-purge`)
- **Codex CLI** — detect + verify `OPENAI_BASE_URL`
- **OpenCode** — patch `~/.config/opencode/config.json` with proxy `baseURL`, merges safely with existing user config
- **VS Code (Cline)** — patch `settings.json` with `cline.apiProvider` + `cline.openAiBaseUrl`
- `tokenmeter install --backend <name>` — target a specific tool
- `tokenmeter verify` — HTTP health check + routing confirmation for all detected tools

---

## Planned — Iteration 5 — OTEL + Prometheus + Gemini (v0.5)

- `plugins/sinks/otel/` — OTLP gRPC exporter
  - Metrics: `llm.tokens.input`, `llm.tokens.output`, `llm.tokens.cached`, `llm.cost.usd`, `llm.latency.ms`
  - Labels: `model`, `provider`, `service_id`
- `plugins/sinks/prometheus/` — `/metrics` scrape endpoint
- **Gemini provider plugin** — native `generativelanguage.googleapis.com` wire format
- Agent-container ephemeral flush (events emitted before sandbox teardown)

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-5)

## Planned — Iteration 6 — GDPR tooling + redaction (v0.6)

- Redaction middleware (PII regex, configurable opt-in)
- SQLite encryption at rest (`TOKENMETER_ENCRYPTION_KEY`)
- `privacy.data_minimisation` mode (strips attribution fields)
- Amazon Bedrock provider plugin

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-6)

## Planned — Iteration 7 — Plugin contribution tooling (v0.7)

- `tokenmeter scaffold` fully implemented for all three plugin types
- Webhook sink — POST `UsageEvent` JSON to any endpoint
- Cost-alert middleware — configurable USD threshold → log + webhook

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-7)

## Planned — Iteration 8 — VS Code extension (v0.8)

- TypeScript extension in `extensions/vscode/`
- Status bar: live session token count + cost
- Webview dashboard: usage by model + cost over time
- Auto-starts daemon if not running
- Cursor + Windsurf backend adapters
- GitHub Copilot interception investigation

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-8)

---

## Contributing

See something missing? [Open an issue](https://github.com/dvdthecoder/tokenmeter/issues/new/choose) or check [CONTRIBUTING.md](https://github.com/dvdthecoder/tokenmeter/blob/main/CONTRIBUTING.md) to submit a plugin.
