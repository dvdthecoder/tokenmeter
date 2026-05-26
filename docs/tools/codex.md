# Codex CLI

Codex CLI reads `OPENAI_BASE_URL`. tokenmeter intercepts all its traffic automatically after `tokenmeter install`.

## Setup

```sh
tokenmeter install   # sets OPENAI_BASE_URL=http://127.0.0.1:4191 in shell profile
```

Open a new shell. Codex routes through tokenmeter with no other configuration.

## Verify

```sh
echo $OPENAI_BASE_URL   # → http://127.0.0.1:4191
codex "explain this file"
tokenmeter query --last 5m
```

If `OPENAI_BASE_URL` is empty, reload your shell profile (`source ~/.zshrc`) or re-run `tokenmeter install --backend codex`.

## What's captured

Codex uses the OpenAI Chat Completions wire format. tokenmeter captures:

| Field | Source |
|---|---|
| `TokensInput` | `usage.prompt_tokens` |
| `TokensOutput` | `usage.completion_tokens` |
| `TokensCached` | `usage.prompt_tokens_details.cached_tokens` |
| `Model` | `model` field |
| `CostUSD` | Estimated from OpenAI model pricing table |

**Streaming:** tokenmeter injects `stream_options: {"include_usage": true}` into every streaming request so the final SSE chunk carries the usage object. Codex does not notice this addition.

## Per-user attribution

Codex does not send a user identifier in its requests. tokenmeter falls back to the OS username (`$USER`). To override, set:

```sh
export TOKENMETER_USER=yourname
```

This appears in `tokenmeter query` output and can be used with `tokenmeter purge --user yourname` for GDPR erasure.

## Azure OpenAI

If Codex is pointed at an Azure OpenAI endpoint via `OPENAI_BASE_URL`, set tokenmeter's upstream to match:

```yaml
# ~/.config/tokenmeter/config.yaml
proxy:
  upstreams:
    openai: https://your-deployment.openai.azure.com
```

Then keep `OPENAI_BASE_URL=http://127.0.0.1:4191` — tokenmeter forwards to Azure.

## Uninstall

```sh
tokenmeter uninstall   # strips OPENAI_BASE_URL from shell profile
```
