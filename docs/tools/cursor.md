# Cursor

Cursor is an AI-first code editor (VS Code fork). tokenmeter intercepts its API traffic via two mechanisms:

- **`ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL`** — set by `tokenmeter install`, routes Cursor's built-in Claude and GPT models through the proxy
- **`http.proxy` in settings.json** — routes HTTPS CONNECT tunnels (e.g. Copilot extension inside Cursor) through tokenmeter's MITM proxy

## Setup

```sh
tokenmeter install   # or: tokenmeter install --backend cursor
```

This sets the base URL env vars and patches `http.proxy` + `http.proxyStrictSSL` in Cursor's `settings.json`.

If you haven't already, install the MITM CA certificate so Cursor trusts tokenmeter's intercepted TLS connections:

```sh
tokenmeter cert install
```

## Verify

```sh
tokenmeter verify
# cursor: OK
```

Or manually:

```sh
grep http.proxy "$HOME/Library/Application Support/Cursor/User/settings.json"
```

## What's captured

| Traffic | How |
|---|---|
| Cursor built-in AI (Claude, GPT) | `ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL` → plain HTTP |
| Copilot extension inside Cursor | `http.proxy` HTTPS CONNECT MITM |
| Any OpenAI-compatible extension | `OPENAI_BASE_URL` |

## Settings file location

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/Cursor/User/settings.json` |
| Linux | `~/.config/Cursor/User/settings.json` |
| Windows | `%APPDATA%\Cursor\User\settings.json` |

## Uninstall

```sh
tokenmeter uninstall   # reverts settings.json and shell env vars
```
