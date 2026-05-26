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
TIME                  SESSION       MODEL              CLIENT                USER     IN    OUT   CACHED    COST
2026-05-18T09:14:22Z  a3f9c2b1e7d8  claude-sonnet-4-6  claude-code-cli@2.1   alice    3     12    30976     $0.009482
```

---

## `tokenmeter install`

Install the daemon as a system service and configure all detected AI tools.

```sh
tokenmeter install
tokenmeter install --backend claudecode   # specific tool only
```

What it does:
1. Writes default config to `~/.config/tokenmeter/config.yaml` (if not present)
2. Installs and starts the system service (launchd on macOS, systemd user unit on Linux)
3. Patches your shell profile with `ANTHROPIC_BASE_URL` and `OPENAI_BASE_URL`
4. Runs `Install()` on every detected AI tool backend (Claude Code, Codex, OpenCode, VS Code)

| Flag | Description |
|---|---|
| `--backend` | Configure only one tool: `claudecode`, `codex`, `opencode`, `vscode` |

---

## `tokenmeter verify`

Check that the proxy is running and each detected AI tool is routing through it.

```sh
tokenmeter verify
```

```
proxy:        OK   (127.0.0.1:4191)
claudecode:   OK
opencode:     FAIL — proxy not detected in OPENAI_BASE_URL or ~/.config/opencode/config.json
```

Run `tokenmeter install` to fix any FAIL entries.

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
tokenmeter purge --before 2026-01-01       # date-range deletion
tokenmeter purge --retention-days 30       # rolling window
tokenmeter purge --user alice              # per-user erasure (GDPR Article 17)
```

| Flag | Description |
|---|---|
| `--user` | Delete all events for a specific user (right-to-erasure) |
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

## `tokenmeter dashboard`

Open the built-in web dashboard in a browser. The proxy must be running.

```sh
tokenmeter dashboard
```

Prints the URL (`http://127.0.0.1:4191/dashboard`) and tries to open it automatically. You can also navigate there manually while the proxy is running.

The dashboard auto-refreshes every 10 seconds. Use the time-window buttons (1 h / 6 h / 24 h / 7 d / 30 d) to change the query window.

---

## `tokenmeter insights`

Generate AI-powered insights from local usage data using a locally running [Ollama](https://ollama.com) SLM. The context sent to Ollama contains only aggregated token counts and costs — never any prompt or response content.

```sh
tokenmeter insights                      # generate + stream to terminal
tokenmeter insights --show               # print the latest stored insight
tokenmeter insights --last 30d           # analyze last 30 days
tokenmeter insights --model qwen3:4b     # use a different Ollama model
```

**Requirements:** Ollama running locally with the model pulled:
```sh
brew install ollama     # macOS
ollama pull llama3.2:3b # default model
```

| Flag | Default | Description |
|---|---|---|
| `--show` | false | Print latest stored insight without generating |
| `--last` | `7d` | Time window to analyze |
| `--model` | `llama3.2:3b` | Ollama model name |
| `--db` | default path | Path to SQLite database |

The latest stored insight is also available at `GET http://localhost:4191/insights/latest` as JSON.

---

## `tokenmeter cert`

Manage the local MITM CA certificate required for GitHub Copilot interception.

```sh
tokenmeter cert install   # generate CA + install to system trust store
tokenmeter cert path      # print the path to the CA certificate
```

### `cert install`

Generates a local ECDSA CA (stored in `~/.local/share/tokenmeter/ca.{key,crt}`) and installs it into the system trust store:

- **macOS** — `security add-trusted-cert` to System keychain (prompts for password)
- **Debian/Ubuntu** — copies to `/usr/local/share/ca-certificates/` and runs `update-ca-certificates`
- **Fedora/Arch** — `trust anchor --store`

After running `tokenmeter install`, the VS Code `settings.json` is also patched with:
```json
{
  "http.proxy": "http://127.0.0.1:4191",
  "http.proxyStrictSSL": false
}
```

This routes GitHub Copilot traffic through the MITM proxy.

---

## `tokenmeter scaffold`

Generate a plugin stub.

```sh
tokenmeter scaffold provider myvendor
tokenmeter scaffold sink webhook
tokenmeter scaffold backend cursor
```
