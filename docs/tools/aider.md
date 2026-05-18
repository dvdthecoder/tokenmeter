# Aider

Aider reads `OPENAI_BASE_URL` (for OpenAI-compatible models) and `ANTHROPIC_BASE_URL` (for Claude models). tokenmeter intercepts both.

## Setup

```sh
tokenmeter install   # sets both base URL env vars
```

## Using Aider with Claude

```sh
aider --model claude-sonnet-4-6
```

Traffic routes through tokenmeter's Anthropic provider — all cache token tiers are captured.

## Using Aider with OpenAI or vLLM

```sh
aider --model gpt-4o
aider --model openai/qwen2.5-coder-32b --openai-api-base http://localhost:8000
```

For local models, set `proxy.upstreams.openai` to your vLLM endpoint. Cost is reported as `$0.00` for self-hosted models.

## Notes

- Aider is listed as experimental in the primary supported tools — the env-var hook works but client detection from the User-Agent is not yet validated
- If you see `client=unknown` in query output, [open an issue](https://github.com/yourorg/tokenmeter/issues/new?template=bug_report.md) with the User-Agent header value
