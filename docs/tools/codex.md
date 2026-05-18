# Codex CLI

Codex CLI reads `OPENAI_BASE_URL`. tokenmeter intercepts all its traffic automatically after `tokenmeter install`.

## Setup

```sh
tokenmeter install   # sets OPENAI_BASE_URL=http://127.0.0.1:4191
```

No other configuration needed.

## Notes

- Codex uses the OpenAI Chat Completions format — covered by the built-in OpenAI provider plugin
- Streaming usage is captured via the injected `stream_options: {include_usage: true}`
- If Codex is pointed at a custom Azure OpenAI endpoint, update `proxy.upstreams.openai` in config
