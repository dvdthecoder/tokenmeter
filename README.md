# tokenmeter

[![CI](https://github.com/dvdthecoder/tokenmeter/actions/workflows/ci.yml/badge.svg)](https://github.com/dvdthecoder/tokenmeter/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

Thin, GDPR-compliant token usage proxy for LLM APIs. Sits between your AI coding tools and the model APIs — captures token counts, cost, and latency without ever storing prompt or response content.

Works with Claude Code, Copilot, Cline, Codex, Aider, OpenCode, Continue.dev — any tool that reads `ANTHROPIC_BASE_URL` or `OPENAI_BASE_URL`.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/dvdthecoder/tokenmeter/main/scripts/install.sh | sh
tokenmeter install
```

Restart your shell. Every AI tool request is now captured automatically.

## Verify

```sh
tokenmeter verify          # check proxy + all detected tools
tokenmeter query --last 1h # view captured events
```

## What you get

- **CLI** — `tokenmeter query`, `purge`, `export`, `insights`
- **VS Code extension** — live status bar token count + cost (`⬡ 1.2k tokens · $0.0042`), dashboard webview with charts, auto-starts daemon
- **OTEL + Prometheus** — push metrics to any collector; Grafana dashboard included
- **On-device insights** — local SLM analysis via Ollama; no data leaves the machine
- **GDPR tooling** — per-user `purge --user`, hashed service IDs, retention auto-purge

## Docs

**[dvdthecoder.github.io/tokenmeter](https://dvdthecoder.github.io/tokenmeter)**

## License

Apache 2.0
