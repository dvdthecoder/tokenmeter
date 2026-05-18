# OpenCode

OpenCode is an OpenAI-compatible TUI that reads `OPENAI_BASE_URL`. tokenmeter intercepts its traffic via that env var — no deep integration required.

## Setup

```sh
tokenmeter install   # sets OPENAI_BASE_URL=http://127.0.0.1:4191
```

Restart your shell, then use OpenCode normally.

## Pointing OpenCode at a vLLM backend

If you run OpenCode against a local vLLM instance, update your config:

```yaml
# ~/.config/tokenmeter/config.yaml
proxy:
  upstreams:
    openai: http://localhost:8000   # your vLLM endpoint
```

Traffic flows: `OpenCode → tokenmeter → vLLM`. Token counts are captured; cost is reported as `$0.00` for self-hosted models (correct — you pay infra, not per-token).

## What's captured

OpenCode uses the OpenAI Chat Completions wire format. tokenmeter captures:

- `prompt_tokens` → `TokensInput`
- `completion_tokens` → `TokensOutput`
- `prompt_tokens_details.cached_tokens` → `TokensCached`

For streaming, tokenmeter injects `stream_options: {include_usage: true}` into the request so the final SSE chunk carries the usage object.
