# Visualize locally

Three ways to see your captured token data, in order of setup complexity:

| Option | Requires | What you get |
|---|---|---|
| [Terminal query](#terminal-query) | Nothing extra | Table / JSON / CSV in the shell |
| [VS Code extension](#vs-code-extension) | tokenmeter VS Code extension | Live status bar + dashboard webview |
| [Grafana dashboard](#grafana-dashboard) | Docker | Full metrics dashboard with charts |

---

## Terminal query

No extra tooling needed — `tokenmeter query` reads directly from the local SQLite database.

```sh
tokenmeter query --last 1h
```

```
TIME                  MODEL              CLIENT                USER          IN     OUT    CACHED    COST
2026-05-26T09:14:22Z  claude-sonnet-4-6  claude-code-cli@2.1   alice         142    75     30976     $0.009572
2026-05-26T09:11:05Z  gpt-4o             codex-cli@1.2         alice         88     32     0         $0.001440
──────────────────────────────────────────────────────────────────────────────────────────────
TOTAL (2)                                                                     230    107    30976     $0.011012
```

### Filters and flags

| Flag | Example | Description |
|---|---|---|
| `--last` | `--last 24h`, `--last 7d` | Time window (default: 24h) |
| `--model` | `--model claude-sonnet-4-6` | Filter by model name |
| `--user` | `--user alice` | Filter by username |
| `--limit` | `--limit 50` | Cap rows returned (default: 500) |
| `--format` | `--format json` | Output format: `table` \| `json` \| `csv` |

### JSON output

Useful for scripting — each row matches the `Row` struct in `internal/storage/sqlite/db.go`:

```sh
tokenmeter query --last 24h --format json | jq '.[] | {model, cost_usd}'
```

### CSV export

```sh
tokenmeter query --last 30d --format csv > usage.csv
```

Or use the dedicated export command (includes all fields):

```sh
tokenmeter export --format csv > usage.csv
tokenmeter export --format json > usage.json
```

---

## Live tail during development

Enable the stdout sink to see every `UsageEvent` printed to the terminal as it arrives — no SQLite query needed:

```yaml
# config.dev.yaml (or ~/.config/tokenmeter/config.yaml)
sinks:
  stdout:
    enabled: true
    options:
      enabled: true
```

Then run the proxy in the foreground:

```sh
./bin/tokenmeter start --config config.dev.yaml
```

Each proxied request prints one line:

```
2026-05-26T09:14:22Z  anthropic  claude-sonnet-4-6  in=142 out=75 cached=30976  $0.009572  1706ms
```

Or tail the background proxy log:

```sh
make dev-logs   # tail /tmp/tokenmeter-dev.log
```

---

## Local SLM insights

`tokenmeter insights` sends your recent usage data to a local [Ollama](https://ollama.com) model and returns a plain-English summary — no data leaves the machine.

### Setup

1. Install Ollama: [ollama.com](https://ollama.com)
2. Pull a model:
   ```sh
   ollama pull llama3.2:3b   # fast, ~2 GB — works well for short analysis
   ```
3. Enable insights in config:
   ```yaml
   insights:
     enabled: true
     ollama_url: "http://localhost:11434"
     model: "llama3.2:3b"
     window_days: 7
   ```

### Generate on demand

```sh
tokenmeter insights generate
```

Example output:

```
Token usage insight (last 7 days)
──────────────────────────────────
Total cost: $1.24 across 83 requests.

claude-sonnet-4-6 accounts for 91% of cost ($1.13). Cache hit rate is 68% —
cache is saving roughly $0.38/day vs uncached pricing.

gpt-4o usage spiked on 2026-05-24 (12 requests, $0.09) — likely a Codex session.
```

### Auto-generate daily

```yaml
insights:
  auto_generate: "daily"   # generates once per day in the background
```

### View stored insights

```sh
tokenmeter insights list     # list past reports
tokenmeter insights show     # print the latest report
```

---

## VS Code extension

The VS Code extension puts live data directly in the editor with no terminal required.

**Status bar** — bottom-right corner shows the last hour:
```
⬡ 1.2k tokens · $0.0042
```

**Dashboard panel** — click the status bar item or run `Tokenmeter: Open Dashboard` from the command palette (`⌘⇧P`):

- Tokens by model (bar chart, last 24 h)
- Cost over time (line chart, last 7 d)
- Recent requests table

See [VS Code + Copilot → tokenmeter Extension](../tools/vscode.md#tokenmeter-vs-code-extension) for install instructions.

---

## Grafana dashboard

For a full metrics dashboard with time-series charts, bring up the local collector stack (requires Docker):

```sh
make dev-up
```

Then:

```sh
make collector-open   # opens http://localhost:3000 (admin / tokenmeter)
```

The pre-built Grafana dashboard shows:

- Total cost and token counts (last 24 h)
- Cost over time per user
- Token rate by model
- P95 latency by provider
- Per-user and per-model cost breakdown

See [Central Collector](collector.md) for the full Docker Compose setup and production deployment options.
