# Roadmap

tokenmeter is built in focused iterations. Each ships a working, tested slice — no half-finished features.

## Status

| Iteration | Version | Theme | Status |
|---|---|---|---|
| 1 | — | Traffic flows, tokens captured | ✅ Done |
| 2 | — | SQLite persistence + query CLI | ✅ Done |
| 3 | — | Daemon + install | ✅ Done |
| 4 | v0.4 | Backend integrations | ✅ Done |
| 5 | v0.5 | OTEL push sink | ✅ Done |
| 6 | v0.6 | Gemini provider | ✅ Done |
| 7 | v0.7 | Per-user attribution | Planned |
| 8 | v0.8 | GitHub Copilot + AWS Bedrock providers | Planned |
| 9 | v0.9 | GDPR tooling + redaction | Planned |
| 10 | v0.10 | Plugin scaffold + webhook + cost alerts | Planned |
| 11 | v0.11 | Local SLM insights (privacy-first, on-device) | Planned |
| 12 | v0.12 | VS Code extension + Cursor + Windsurf | Planned |

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

## ✅ Iteration 5 — OTEL push sink (v0.5)

- `plugins/sinks/otel/` — OTLP gRPC exporter to central collector
  - Metrics: `llm.tokens.input`, `llm.tokens.output`, `llm.tokens.cached`, `llm.cost.usd`, `llm.latency.ms`
  - Attributes: `model`, `provider`, `user`
  - Config: `endpoint`, `insecure`, `timeout_ms`, `interval_s`
  - Testable without a real collector (ManualReader-backed test suite)
- Edge-based collection model: SQLite at edge (per-machine), OTEL push to central

## ✅ Iteration 6 — Gemini provider (v0.6)

- `plugins/providers/gemini/` — native `generativelanguage.googleapis.com` wire format
- Detects host, parses `usageMetadata` from SSE (final chunk wins) and REST
- Pricing table for 7 Gemini models; cached tokens billed at 25% of input price
- Unknown models fall back to gemini-2.0-flash pricing

## Planned — Iteration 7 — Per-user attribution (v0.7)

- Pseudonymous user IDs: `SHA-256(username + org_salt)`
- Config: `privacy.hash_user`, `privacy.hash_hostname`, `privacy.org_salt`
- `UsageEvent.UserID` field (resolved at proxy ingestion, never raw PII in metrics store)
- `tokenmeter purge --user <name>` per-user GDPR erasure
- Identity mapping maintained separately from metrics

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-7)

## Planned — Iteration 8 — GitHub Copilot + AWS Bedrock (v0.8)

Enterprise model coverage — the two largest sources of LLM traffic not yet captured:

- **GitHub Copilot** — intercept investigation (env var spike → VS Code extension wrapper if needed); wire format parser for `api.githubcopilot.com`
- **AWS Bedrock** — Converse API provider plugin (`bedrock-runtime.<region>.amazonaws.com`); SigV4 transparent pass-through; cost table for Claude/Titan/Llama/Mistral on Bedrock
- `tokenmeter verify` updated to show Copilot routing status

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-8)

## Planned — Iteration 9 — GDPR tooling + redaction (v0.9)

- Redaction middleware (PII regex, configurable opt-in)
- SQLite encryption at rest (`TOKENMETER_ENCRYPTION_KEY`)
- `privacy.data_minimisation` mode (strips attribution fields)

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-9)

## Planned — Iteration 10 — Plugin scaffold + webhook + cost alerts (v0.10)

- `tokenmeter scaffold` fully implemented for all four plugin types (provider, sink, middleware, backend)
- Webhook sink — POST `UsageEvent` JSON to any endpoint
- Cost-alert middleware — configurable USD threshold → log + webhook
- Prometheus `/metrics` scrape endpoint (optional, for shared deployment)

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-10)

## Planned — Iteration 11 — Local SLM insights (v0.11)

Privacy-first, on-device analysis — no data leaves the machine:

- `tokenmeter insights` — runs a local SLM via Ollama against SQLite data
- Natural-language cost pattern analysis, cache efficiency advice, model right-sizing suggestions
- `tokenmeter insights --last 7d --format markdown` — weekly digest
- Context builder: aggregated counts + costs only, never raw prompts/responses
- Config: `insights.ollama_url`, `insights.model`, `insights.context_rows`
- **Prerequisite before productionising central streaming** — gives every user immediate value from local data without needing any infra

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-11)

## Planned — Iteration 12 — VS Code extension + Cursor + Windsurf (v0.12)

- TypeScript extension in `extensions/vscode/`
- Status bar: live session token count + cost
- Webview dashboard: usage by model + cost over time
- Auto-starts daemon if not running
- Cursor + Windsurf backend adapters

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-12)

---

## Contributing

See something missing? [Open an issue](https://github.com/dvdthecoder/tokenmeter/issues/new/choose) or check [CONTRIBUTING.md](https://github.com/dvdthecoder/tokenmeter/blob/main/CONTRIBUTING.md) to submit a plugin.
