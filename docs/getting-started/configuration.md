# Configuration

The config file lives at `~/.config/tokenmeter/config.yaml`. It is created with sensible defaults by `tokenmeter install` and is never overwritten by upgrades.

## Full reference

```yaml
proxy:
  listen: "127.0.0.1:4191"   # address the proxy binds to
  mode: sidecar               # sidecar (one per user) | shared (team proxy)
  service_id: ""              # optional label for this machine in metrics

  # Override upstream base URLs per provider.
  # Useful for Azure OpenAI, vLLM, or LiteLLM endpoints.
  upstreams:
    anthropic: https://api.anthropic.com
    openai: https://api.openai.com

sinks:
  stdout:
    enabled: false            # echo events to stderr (dev/debug mode)
    options:
      enabled: false

  sqlite:
    enabled: true
    options:
      path: ""                # default: ~/.local/share/tokenmeter/events.db
      retention_days: 90      # auto-purge events older than this on startup

  # otel:                     # coming in v0.5
  #   enabled: false
  #   options:
  #     endpoint: localhost:4317
  #     insecure: true

privacy:
  hash_service_id: true       # SHA-256 hash service_id before storage
  encrypt_at_rest: false      # SQLite encryption (requires TOKENMETER_ENCRYPTION_KEY)
  encryption_key: ""          # prefer the env var instead

retention:
  days: 90                    # global default, overridden per sink
```

## Environment variables

| Variable | Description |
|---|---|
| `ANTHROPIC_BASE_URL` | Set by `tokenmeter install` — points Claude Code at the proxy |
| `OPENAI_BASE_URL` | Set by `tokenmeter install` — points OpenAI-compatible tools at the proxy |
| `TOKENMETER_USER` | Override the username attributed to events (default: `$USER`) |
| `TOKENMETER_ENCRYPTION_KEY` | SQLite encryption key (32 hex chars) |

## Pointing at custom upstreams

=== "Azure OpenAI"

    ```yaml
    proxy:
      upstreams:
        openai: https://my-resource.openai.azure.com
    ```

=== "vLLM (self-hosted)"

    ```yaml
    proxy:
      upstreams:
        openai: http://localhost:8000
    ```

=== "LiteLLM proxy"

    ```yaml
    proxy:
      upstreams:
        openai: http://localhost:4000
        anthropic: http://localhost:4000
    ```

## Enabling stdout sink (dev mode)

Useful when running `tokenmeter start` in a terminal to see events as they arrive:

```yaml
sinks:
  stdout:
    enabled: true
    options:
      enabled: true
```

Or pass it at runtime without editing the file — `tokenmeter start` enables stdout automatically.
