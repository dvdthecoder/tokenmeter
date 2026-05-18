# tokenmeter

[![CI](https://github.com/yourorg/tokenmeter/actions/workflows/ci.yml/badge.svg)](https://github.com/yourorg/tokenmeter/actions/workflows/ci.yml)
[![Go 1.23+](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://go.dev)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

Thin, GDPR-compliant token usage meter for LLM APIs. Sits between your AI coding tools and the model APIs — captures token counts, cost, and latency **without ever storing prompt or response content**.

```
Claude Code / OpenCode / Aider / VS Code
        ↓
  tokenmeter  (127.0.0.1:4191)
        ↓
  Anthropic / OpenAI / vLLM / Azure
        ↓
  UsageEvent — model · tokens · cost · latency
        ↓
  SQLite  ·  OTEL  ·  Prometheus
```

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/yourorg/tokenmeter/main/scripts/install.sh | sh
tokenmeter install
```

Restart your shell. Done — every AI tool request is now captured.

## Query

```sh
tokenmeter query --last 24h
tokenmeter query --last 7d --format json
tokenmeter export --format csv > usage.csv
```

## Claude Code skills

Inside Claude Code, use:

```
/proxy-status    daemon health + recent events
/proxy-report    token and cost summary
/proxy-purge     GDPR data deletion
```

## Tool coverage

| Tool | Status |
|---|---|
| Claude Code CLI | ✅ |
| OpenCode | ✅ |
| Aider | ✅ |
| Codex CLI | ✅ |
| Continue.dev (VS Code) | ✅ |
| Cline (VS Code) | ✅ |
| GitHub Copilot | 🔨 v0.8 |
| Gemini CLI | 🔨 v0.5 |

## Plugin architecture

Add support for any provider, sink, or AI tool — no core changes needed:

```sh
tokenmeter scaffold provider gemini
tokenmeter scaffold sink webhook
tokenmeter scaffold backend cursor
```

See the [plugin guide →](https://yourorg.github.io/tokenmeter/plugins/overview/)

## GDPR

- Content-blind — `UsageEvent` contains only metadata
- Local-first — SQLite on your machine, zero cloud dependencies
- Right to erasure — `tokenmeter purge --before <date>`
- Auto-purge — configurable retention (default 90 days)
- No telemetry — zero outbound calls except to your LLM API

See [Privacy & GDPR docs →](https://yourorg.github.io/tokenmeter/privacy/)

## Docs

**[yourorg.github.io/tokenmeter](https://yourorg.github.io/tokenmeter)**

- [Getting started](https://yourorg.github.io/tokenmeter/getting-started/installation/)
- [Configuration](https://yourorg.github.io/tokenmeter/getting-started/configuration/)
- [CLI reference](https://yourorg.github.io/tokenmeter/cli/)
- [Roadmap](https://yourorg.github.io/tokenmeter/roadmap/)

## Contributing

[CONTRIBUTING.md](CONTRIBUTING.md) · [Open an issue](https://github.com/yourorg/tokenmeter/issues/new/choose) · [Plugin guide](https://yourorg.github.io/tokenmeter/plugins/overview/)

## License

Apache 2.0
