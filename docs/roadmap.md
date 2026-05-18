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
| 7 | v0.7 | Per-user value | ✅ Done |
| 8 | v0.8 | GitHub Copilot + AWS Bedrock providers | ✅ Done |
| 9 | v0.9 | Local SLM insights (on-device, Ollama) | Planned |
| 10 | v0.10 | VS Code extension + Cursor + Windsurf surface | Planned |
| 11 | v0.11 | Central collection hardening + GDPR facade | Planned |
| 12 | v0.12 | Plugin scaffold + webhook + cost alerts | Planned |

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

## ✅ Iteration 7 — Per-user value (v0.7)

- OS user resolution: `TOKENMETER_USER` → `USER` → `USERNAME` → hostname → `"unknown"`
- `tokenmeter query --user <name>` — filter events by user
- `tokenmeter purge --user <name>` — GDPR right-to-erasure per individual
- Optional pseudonymisation: `privacy.hash_user: true` → `SHA-256(username + org_salt)`; salt via `TOKENMETER_ORG_SALT` env var; default off

## ✅ Iteration 8 — GitHub Copilot + AWS Bedrock (v0.8)

Enterprise model coverage — the two largest sources of LLM traffic not yet captured:

- **GitHub Copilot** — HTTP CONNECT MITM interception (`internal/mitm/`); ECDSA local CA with on-demand per-host cert signing; VS Code `http.proxy` + `http.proxyStrictSSL: false` patched automatically by `tokenmeter install`; `tokenmeter cert install` adds CA to system trust store (macOS, Debian, Fedora)
- **AWS Bedrock** — Converse API + InvokeModelWithResponseStream provider plugin; detects `*.bedrock.amazonaws.com`; cost table for Claude, Llama, Mistral, Nova on Bedrock; transparent SigV4 pass-through
- Copilot provider delegates to OpenAI-compatible wire format; cost always 0 (subscription)
- OpenAI plugin guard-rails: explicit exclusion of Copilot + Bedrock hosts so dedicated plugins always win

## Planned — Iteration 9 — Local SLM insights (v0.9)

**Generate → store → surface.** Insights are persistent, not ephemeral terminal output:

- `tokenmeter insights` — Ollama reads SQLite, generates insight, stores to `insights` table, streams to terminal
- `GET /insights/latest` — lightweight HTTP endpoint so Grafana dashboard can poll and display
- Grafana dashboard updated with an **Insights** text panel (latest stored insight, auto-refreshes)
- `auto_generate: daily` — daemon generates an insight once per day in the background
- Context builder sends only aggregated counts + costs — never prompts or responses
- Graceful skip if Ollama not running

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-9)

## Planned — Iteration 10 — VS Code extension + Cursor + Windsurf surface (v0.10)

- TypeScript extension in `extensions/vscode/`
- Status bar: live session token count + estimated cost
- Webview dashboard: usage by model + cost over time (reads local SQLite)
- Auto-starts daemon if not running
- Cursor + Windsurf backend adapters

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-10)

## Planned — Iteration 11 — Central collection hardening + GDPR facade (v0.11)

GDPR tooling is sequenced here because it's the privacy layer applied *when* data is productionised to flow to a shared central collector — not before. The facade pattern: obfuscate at the edge before anything leaves.

- Redaction middleware (PII regex, configurable opt-in) — strips content before OTEL push
- SQLite encryption at rest (`TOKENMETER_ENCRYPTION_KEY`) — edge data at rest
- `privacy.data_minimisation` mode — strips attribution fields, keeps counts + cost only
- Central collector production hardening — TLS, auth, retention policies
- Prometheus `/metrics` scrape endpoint (for shared/team deployments)

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-11)

## Planned — Iteration 12 — Plugin scaffold + webhook + cost alerts (v0.12)

- `tokenmeter scaffold` fully implemented for all four plugin types
- Webhook sink — POST `UsageEvent` JSON to any endpoint
- Cost-alert middleware — configurable USD threshold → log + webhook

[Open issues →](https://github.com/dvdthecoder/tokenmeter/issues?q=label%3Aiteration-12)

---

## Contributing

See something missing? [Open an issue](https://github.com/dvdthecoder/tokenmeter/issues/new/choose) or check [CONTRIBUTING.md](https://github.com/dvdthecoder/tokenmeter/blob/main/CONTRIBUTING.md) to submit a plugin.
