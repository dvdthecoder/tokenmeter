# CLI Reference

## `tokenmeter start`

Start the proxy in the foreground (dev mode). Enables stdout and SQLite sinks by default.

```sh
tokenmeter start
tokenmeter start --config ~/.config/tokenmeter/config.yaml
```

| Flag | Default | Description |
|---|---|---|
| `--config` | — | Path to config.yaml |

---

## `tokenmeter daemon`

Start the proxy as a detached background process. Logs go to the platform log file.

```sh
tokenmeter daemon
tokenmeter daemon --config ~/.config/tokenmeter/config.yaml
```

**Log location:**
- macOS: `~/Library/Application Support/tokenmeter/tokenmeter.log`
- Linux: `~/.local/share/tokenmeter/tokenmeter.log`

---

## `tokenmeter stop`

Send SIGTERM to the running daemon and wait up to 10 s for a clean exit.

```sh
tokenmeter stop
```

---

## `tokenmeter status`

Show whether the daemon is running and print the last 5 captured events.

```sh
tokenmeter status
```

```
status:  running (pid 12345)
log:     ~/Library/Application Support/tokenmeter/tokenmeter.log

recent events:
TIME                  MODEL              CLIENT                USER     IN    OUT   CACHED    COST
2026-05-18T09:14:22Z  claude-sonnet-4-6  claude-code-cli@2.1   alice    3     12    30976     $0.009482
```

---

## `tokenmeter install`

Install the daemon as a system service and configure detected AI tools.

```sh
tokenmeter install
tokenmeter install --backend claudecode   # specific tool only (v0.4+)
```

What it does:
1. Writes default config to `~/.config/tokenmeter/config.yaml` (if not present)
2. Installs and starts the system service (launchd / systemd)
3. Patches shell profile with `ANTHROPIC_BASE_URL` and `OPENAI_BASE_URL`

---

## `tokenmeter uninstall`

Remove the system service and revert shell configuration.

```sh
tokenmeter uninstall
```

Your SQLite database is preserved. Run `tokenmeter purge` first to wipe it.

---

## `tokenmeter query`

Query captured events from the local SQLite database.

```sh
tokenmeter query
tokenmeter query --last 1h
tokenmeter query --last 7d --model claude-sonnet-4-6
tokenmeter query --last 30d --format json
tokenmeter query --format csv > report.csv
```

| Flag | Default | Description |
|---|---|---|
| `--last` | `24h` | Time window: `1h`, `6h`, `24h`, `7d`, `30d` |
| `--format` | `table` | Output format: `table`, `json`, `csv` |
| `--model` | — | Filter by model name |
| `--user` | — | Filter by username |
| `--limit` | `500` | Max rows (0 = unlimited) |
| `--db` | default path | Path to SQLite database |

---

## `tokenmeter purge`

GDPR-compliant event deletion.

```sh
tokenmeter purge --before 2026-01-01
tokenmeter purge --retention-days 30
```

| Flag | Description |
|---|---|
| `--before` | Delete events before this date (YYYY-MM-DD or RFC3339) |
| `--retention-days` | Delete events older than N days |
| `--db` | Path to SQLite database |

---

## `tokenmeter export`

Export all events to stdout.

```sh
tokenmeter export                    # JSON (default)
tokenmeter export --format csv       # CSV
tokenmeter export --format csv > events.csv
```

| Flag | Default | Description |
|---|---|---|
| `--format` | `json` | Output format: `json`, `csv` |
| `--db` | default path | Path to SQLite database |

---

## `tokenmeter scaffold`

Generate a plugin stub.

```sh
tokenmeter scaffold provider gemini
tokenmeter scaffold sink webhook
tokenmeter scaffold backend cursor
```

Creates the implementation file with the interface pre-filled.
