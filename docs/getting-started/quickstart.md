# Quickstart

This gets you from zero to your first captured token event in under 5 minutes.

## 1. Install and start

```sh
curl -fsSL https://raw.githubusercontent.com/yourorg/tokenmeter/main/scripts/install.sh | sh
tokenmeter install
```

## 2. Open a new shell

The env vars are set in your RC file — you need a fresh shell for them to take effect.

```sh
echo $ANTHROPIC_BASE_URL   # → http://127.0.0.1:4191
echo $OPENAI_BASE_URL      # → http://127.0.0.1:4191
```

## 3. Use your AI tool normally

Run Claude Code, OpenCode, Aider, or any supported tool as you normally would. tokenmeter is transparent — the tool doesn't know it's being proxied.

```sh
claude "what is 2+2"
```

## 4. Query what was captured

```sh
tokenmeter query --last 1h
```

```
TIME                  MODEL              CLIENT                USER          IN    OUT   CACHED    COST
2026-05-18T09:14:22Z  claude-sonnet-4-6  claude-code-cli@2.1   alice         3     12    30976     $0.009482
──────────  ────────                                                                    
TOTAL (1)                                                       3     12    30976     $0.009482
```

## 5. Export or purge

```sh
# Export everything as CSV
tokenmeter export --format csv > usage.csv

# GDPR purge — delete events older than 30 days
tokenmeter purge --retention-days 30
```

## What's happening under the hood

```
Your shell  ──(ANTHROPIC_BASE_URL set)──►  claude CLI
                                                │
                                    POST /v1/messages
                                                │
                                        ┌───────▼────────┐
                                        │  tokenmeter     │  127.0.0.1:4191
                                        │  (reverse proxy)│
                                        └───────┬────────┘
                                                │  forwards request
                                        ┌───────▼────────┐
                                        │  api.anthropic  │
                                        │  .com           │
                                        └───────┬────────┘
                                                │  SSE stream
                                        ┌───────▼────────┐
                                        │  tokenmeter     │  intercepts usage
                                        │  (stream parser)│  from message_delta
                                        └───────┬────────┘
                                                │
                                          UsageEvent{}
                                                │
                                         SQLite sink
```

The SSE stream is forwarded to the caller in real time — there is zero added latency for token delivery. Usage is extracted from the terminal SSE event after the stream completes.
