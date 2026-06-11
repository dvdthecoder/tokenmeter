# tokenmeter

**Thin, GDPR-compliant token usage meter for LLM APIs.**

tokenmeter sits between your AI coding tools and the model APIs, capturing token usage metadata — without ever storing prompt or response content.

```
Claude Code / Copilot / OpenCode / Aider / VS Code
        ↓
  tokenmeter  (127.0.0.1:4191)
        ↓
  Anthropic / OpenAI / Gemini / Copilot / Bedrock / vLLM
        ↓
  UsageEvent  ← model · tokens · cost · latency
        ↓
  SQLite · OTEL · Prometheus
        ↓
  tokenmeter insights  ← local SLM analysis (Ollama, no cloud)
```

---

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/dvdthecoder/tokenmeter/main/scripts/install.sh | sh
tokenmeter install
tokenmeter verify
```

`install` detects your AI tools, registers the daemon as a system service, patches your shell profile, and configures each tool's settings. `verify` confirms everything is routing correctly.

---

## What it captures

Every proxied request produces one `UsageEvent`:

| Field | Example |
|---|---|
| Model | `claude-sonnet-4-6` |
| Input tokens | `142` |
| Output tokens | `75` |
| Cached tokens (read) | `30 976` |
| Cache write tokens | `500` |
| Estimated cost | `$0.009572` |
| Latency | `1 706 ms` |
| Client | `claude-code-cli@2.1.142` |
| User | `abhishekdwivedi` |
| Session ID | `a3f9c2b1e7d8…` (from request headers, groups a conversation) |

Prompts and responses are **never stored**.

---

## Tool coverage

| Tool | Hook | Status |
|---|---|---|
| Claude Code CLI | `ANTHROPIC_BASE_URL` + skills | ✅ |
| OpenCode | `OPENAI_BASE_URL` + `config.json` | ✅ |
| Aider | `OPENAI_BASE_URL` / `ANTHROPIC_BASE_URL` | ✅ |
| Codex CLI | `OPENAI_BASE_URL` | ✅ |
| Continue.dev (VS Code) | `OPENAI_BASE_URL` | ✅ |
| Cline (VS Code) | `settings.json` patch | ✅ |
| Cursor | `settings.json` `http.proxy` | ✅ |
| Windsurf | `settings.json` `http.proxy` | ✅ |
| Gemini CLI | native Gemini API | ✅ |
| GitHub Copilot | MITM proxy + local CA | ✅ |
| **VS Code Extension** | status bar + dashboard webview | ✅ |

## Provider coverage

| Provider | Wire format | Cost table |
|---|---|---|
| Anthropic | SSE + REST, all cache tiers | ✅ |
| OpenAI | SSE (usage injection) + REST | ✅ |
| Google Gemini | SSE + REST `usageMetadata` | ✅ |
| GitHub Copilot | OpenAI-compatible (HTTPS MITM) | ✅ (subscription = $0) |
| AWS Bedrock | Converse API + InvokeModelWithResponseStream | ✅ |
| OpenAI-compatible | vLLM, LiteLLM, Ollama, OpenCode runner | ✅ |

---

## Key properties

- **Content-blind** — `UsageEvent` contains only metadata, never text
- **Local-first** — SQLite on your machine, zero cloud dependencies
- **Tool-agnostic** — works via env var override, no SDK required
- **Plugin architecture** — add providers, sinks, backends without touching core code
- **GDPR-ready** — `service_id` hashed, per-user right to erasure via `tokenmeter purge --user`
- **On-device insights** — `tokenmeter insights` uses a local Ollama SLM for cost analysis; no data leaves the machine
- **VS Code extension** — live status bar token count + cost, usage dashboard webview, auto-starts daemon
- **`tokenmeter top`** — abtop-inspired live terminal dashboard; polls SQLite every 2 s, vim-style scroll, proxy health indicator, `--json` flag for scripting

[Get started →](getting-started/installation.md){ .md-button .md-button--primary }
[Roadmap →](roadmap.md){ .md-button }
