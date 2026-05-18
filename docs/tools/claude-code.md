# Claude Code

Claude Code CLI (`claude`) uses `ANTHROPIC_BASE_URL` to route API calls. tokenmeter intercepts that traffic with zero changes to how you use Claude Code.

## Setup

`tokenmeter install` handles everything automatically:

```sh
tokenmeter install
tokenmeter verify   # confirm routing
```

To verify manually:

```sh
echo $ANTHROPIC_BASE_URL    # → http://127.0.0.1:4191
claude "hello"
tokenmeter query --last 5m
```

## In-tool commands (Claude Code skills)

`tokenmeter install` copies three skill files into `~/.claude/skills/`:

| Command | What it does |
|---|---|
| `/proxy-status` | Daemon health + last 5 events |
| `/proxy-report` | Token and cost summary |
| `/proxy-purge` | GDPR data deletion |

These are available immediately inside any Claude Code session after install.

## What tokenmeter captures

Claude Code sends a `User-Agent` header of the form `claude-cli/2.1.142 (external, sdk-cli)`. tokenmeter parses this to populate:

- `ClientName`: `claude-code-cli`
- `ClientVersion`: e.g. `2.1.142`

## Anthropic-specific fields captured

| Field | Source SSE event | Notes |
|---|---|---|
| `TokensInput` | `message_start` | Fresh input tokens |
| `TokensCached` | `message_start` | Cache-read tokens (10% cost) |
| `TokensCachedCreation` | `message_start` | Cache-write tokens (125% cost) |
| `TokensOutput` | `message_delta` | Generated tokens |
| `ServiceTier` | `message_start` | `standard` or `priority` |
| `InferenceGeo` | `message_start` | Region where inference ran |

## Health check

Claude Code sends `HEAD /` before each request to verify the proxy is alive. tokenmeter returns `200 OK` and suppresses log noise for that path.
