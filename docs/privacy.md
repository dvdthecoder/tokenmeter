# Privacy & GDPR

## What tokenmeter stores

tokenmeter is **content-blind by design**. A `UsageEvent` contains only metadata:

| Field | Example | Stored? |
|---|---|---|
| Model name | `claude-sonnet-4-6` | ✅ |
| Token counts | `in=142 out=75 cached=30976` | ✅ |
| Estimated cost | `$0.009572` | ✅ |
| Latency | `1706 ms` | ✅ |
| Timestamp | `2026-05-18T09:14:22Z` | ✅ |
| Client name | `claude-code-cli@2.1.142` | ✅ |
| Username | `alice` | ✅ (hashed optional) |
| Service ID | SHA-256 prefix | ✅ (hashed by default) |
| **Prompt text** | — | ❌ Never |
| **Response text** | — | ❌ Never |
| **API keys** | — | ❌ Never |

## Invariants

These hold regardless of configuration:

1. **No content stored.** The proxy reads SSE event types and usage fields only. Prompt and response content bytes are forwarded to the caller and never written to any buffer beyond what's needed for TCP framing.

2. **No outbound connections.** The tokenmeter binary connects only to the upstream model API (as configured) and, optionally, an OTEL collector you specify. It makes no calls to Anthropic's or OpenAI's analytics endpoints.

3. **Local-first.** The SQLite database lives on the developer's machine. Nothing is sent to any cloud service unless you configure an OTEL sink pointing at one.

## Service ID hashing

The `service_id` field (set in config) identifies this machine or agent. It is SHA-256 hashed before storage when `privacy.hash_service_id: true` (the default), producing a 16-character hex prefix:

```
service_id: "alice-macbook" → stored as "3f4a9c2b1e7d8f0a"
```

The original value is never stored. This makes it impossible to reverse-engineer the hostname from the database.

## User attribution

Every event is stamped with the OS user at proxy ingestion time. Resolution order:

```
TOKENMETER_USER env var → USER → USERNAME (Windows) → hostname → "unknown"
```

### Optional pseudonymisation

When `privacy.hash_user: true`, the username is replaced with `SHA-256(username + org_salt)` before reaching any sink. The original name is never stored.

```yaml
privacy:
  hash_user: true     # default: false
  org_salt: ""        # set via TOKENMETER_ORG_SALT env var — shared across team machines
```

```sh
export TOKENMETER_ORG_SALT=your-shared-team-secret
```

The `org_salt` prevents cross-org correlation of the same username. Without it, two orgs with a user named `alice` would produce the same hash.

## Right to erasure (GDPR Article 17)

```sh
# Per-user erasure — deletes all events for one person
tokenmeter purge --user alice

# Date-range deletion
tokenmeter purge --before 2026-01-01

# Rolling retention
tokenmeter purge --retention-days 30
```

Auto-purge runs at daemon startup based on `retention.days` in config (default: 90 days).

## Encryption at rest

SQLite encryption is available but disabled by default:

```yaml
privacy:
  encrypt_at_rest: true
  # encryption_key: ""   # set TOKENMETER_ENCRYPTION_KEY env var instead
```

```sh
export TOKENMETER_ENCRYPTION_KEY=your-32-char-hex-key
tokenmeter start
```

## Shared proxy mode

In `mode: shared`, multiple developers' events land in the same database. Recommendations:

- Enable `hash_service_id: true` (default)
- Set `TOKENMETER_USER` per developer so events are attributed correctly
- Run on a machine controlled by the team, not an individual's laptop
- Apply filesystem permissions so only authorised users can read the SQLite file

## Data minimisation mode

Planned for v0.11 (central collection hardening): `privacy.data_minimisation: true` will drop `username`, `client_name`, and `client_version` from every event before it reaches any sink — useful when pushing to a shared central collector.
