# OpenCode

OpenCode is an OpenAI-compatible TUI. tokenmeter intercepts its traffic via `OPENAI_BASE_URL` and also patches `~/.config/opencode/config.json` directly.

## Setup

```sh
tokenmeter install
tokenmeter verify
```

Restart your shell, then use OpenCode normally.

## What install does

`tokenmeter install` does two things for OpenCode:

1. Sets `OPENAI_BASE_URL=http://127.0.0.1:4191` in your shell profile
2. Patches `~/.config/opencode/config.json` with:

```json
{
  "providers": {
    "openai":     { "baseURL": "http://127.0.0.1:4191" },
    "anthropic":  { "baseURL": "http://127.0.0.1:4191" }
  }
}
```

Existing keys in `config.json` (e.g. `apiKey`, `theme`) are preserved — only `baseURL` is added or updated.

To target OpenCode only:

```sh
tokenmeter install --backend opencode
```

## Pointing OpenCode at a vLLM backend

If you run OpenCode against a local vLLM instance, update your tokenmeter config:

```yaml
# ~/.config/tokenmeter/config.yaml
proxy:
  upstreams:
    openai: http://localhost:8000   # your vLLM endpoint
```

Traffic flows: `OpenCode → tokenmeter → vLLM`. Token counts are captured; cost is reported as `$0.00` for self-hosted models.

## What's captured

OpenCode uses the OpenAI Chat Completions wire format. tokenmeter captures:

- `prompt_tokens` → `TokensInput`
- `completion_tokens` → `TokensOutput`
- `prompt_tokens_details.cached_tokens` → `TokensCached`

For streaming, tokenmeter injects `stream_options: {include_usage: true}` so the final SSE chunk carries the usage object.
