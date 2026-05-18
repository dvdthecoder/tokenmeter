# tokenmeter

A thin, GDPR-compliant token usage meter for LLM APIs. Intercepts traffic from AI coding tools, extracts token usage metadata, and emits it via OpenTelemetry or local SQLite — without ever storing prompt content.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/yourorg/tokenmeter/main/scripts/install.sh | sh
tokenmeter install
```

`install` detects which AI tools you have (Claude Code, Codex CLI, OpenCode, VS Code extensions) and configures them automatically.

## Claude Code

```
/install-proxy    install and configure
/proxy-status     daemon health + recent events
/proxy-report     token usage and cost summary
/proxy-purge      GDPR data deletion
```

## How it works

```
AI Tool → tokenmeter (127.0.0.1:4191) → LLM API
                ↓
         UsageEvent (metadata only)
                ↓
         SQLite / OTEL / Prometheus
```

Prompts and responses are never stored. Only: model, tokens in/out/cached, latency, estimated cost, timestamp.

## Plugin contribution

Add a provider, sink, middleware, or backend adapter:

```sh
tokenmeter scaffold provider <name>
tokenmeter scaffold sink <name>
tokenmeter scaffold backend <name>
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/plugin-guide.md](docs/plugin-guide.md).

## GDPR

- **Local-first**: data stays on your machine
- **Content-blind**: no prompts, no responses
- **Retention**: auto-purge after configurable days (default 90)
- **Erasure**: `tokenmeter purge --before <date>` or `--service-id <id>`
- **No telemetry**: the binary makes zero outbound calls except to your LLM API and configured OTEL collector

## License

Apache 2.0
