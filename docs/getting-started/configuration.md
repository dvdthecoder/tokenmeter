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

  otel:
    enabled: false
    options:
      endpoint: "localhost:4317"   # OTLP gRPC endpoint
      insecure: true
      interval_s: 60

  prometheus:
    enabled: false
    options:
      listen: "127.0.0.1:9090"

privacy:
  hash_service_id: true       # SHA-256 hash service_id before storage (default: true)
  hash_user: false            # pseudonymise username: SHA-256(username + org_salt)
  org_salt: ""                # shared team salt; prefer TOKENMETER_ORG_SALT env var
  data_minimisation: false    # strip all attribution fields (username, client, session, service_id)
  encrypt_at_rest: false      # SQLite field-level encryption (AES-256-GCM)
  encryption_key: ""          # prefer TOKENMETER_ENCRYPTION_KEY env var

pricing:
  remote_fallback: false      # fetch prices for unknown models from models.dev
  cache_path: ""              # default: ~/.local/share/tokenmeter/pricing-cache.json

middleware: []                # optional chain of transform/gate plugins (runs before sinks)
# middleware:
#   - name: redaction
#     options:
#       patterns: ['\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b']
#       fields: [username, service_id]

retention:
  days: 90                    # auto-purge events older than this on startup

insights:
  enabled: false
  ollama_url: "http://localhost:11434"  # local Ollama endpoint
  model: "llama3.2:3b"                  # any model pulled in Ollama
  auto_generate: ""                     # "daily" to auto-generate in background
  window_days: 7                        # days of events to analyse
```

## Environment variables

| Variable | Description |
|---|---|
| `ANTHROPIC_BASE_URL` | Set by `tokenmeter install` — points Claude Code at the proxy |
| `OPENAI_BASE_URL` | Set by `tokenmeter install` — points OpenAI-compatible tools at the proxy |
| `TOKENMETER_USER` | Override the username attributed to events (default: `$USER`) |
| `TOKENMETER_ENCRYPTION_KEY` | SQLite encryption key (32 hex chars) |
| `TOKENMETER_ORG_SALT` | Shared salt for user pseudonymisation across team machines |

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
