# Claude Code

Claude Code CLI (`claude`) uses `ANTHROPIC_BASE_URL` to route API calls. tokenmeter intercepts that traffic with zero changes to how you use Claude Code.

## Setup

`tokenmeter install` handles everything automatically. To verify:

```sh
echo $ANTHROPIC_BASE_URL    # → http://127.0.0.1:4191
claude "hello"              # → events appear in tokenmeter query
```

## In-tool commands (Claude Code skills)

tokenmeter installs a set of skills into `~/.claude/skills/` that you can invoke from inside Claude Code:

| Command | What it does |
|---|---|
| `/proxy-status` | Daemon health + last 5 events |
| `/proxy-report` | Token and cost summary for the current session |
| `/proxy-purge` | GDPR data deletion |
| `/install-proxy` | Install or reinstall tokenmeter |
| `/add-provider` | Scaffold a new provider plugin |
| `/add-sink` | Scaffold a new sink plugin |
| `/add-backend` | Scaffold a new backend adapter |

## What tokenmeter captures

Claude Code sends a `User-Agent` header of the form `claude-cli/2.1.142 (external, sdk-cli)`. tokenmeter parses this to populate:

- `ClientName`: `claude-code-cli` or `claude-code-app`
- `ClientVersion`: e.g. `2.1.142`

The `(external, sdk-cli)` suffix indicates the auth mode — `external` means an API key is in use rather than a claude.ai subscription.

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

Claude Code sends `HEAD /` before each request to check the proxy is alive. tokenmeter returns `200 OK` and suppresses the log noise for that path.
