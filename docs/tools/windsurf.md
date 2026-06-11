# Windsurf

Windsurf (by Codeium) is an AI-first code editor (VS Code fork). tokenmeter intercepts its API traffic via two mechanisms:

- **`ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL`** — set by `tokenmeter install`, routes Windsurf's AI extension API calls through the proxy
- **`http.proxy` in settings.json** — routes HTTPS CONNECT tunnels through tokenmeter's MITM proxy

## Setup

```sh
tokenmeter install   # or: tokenmeter install --backend windsurf
```

This sets the base URL env vars and patches `http.proxy` + `http.proxyStrictSSL` in Windsurf's `settings.json`.

Install the MITM CA so Windsurf trusts tokenmeter's intercepted TLS connections:

```sh
tokenmeter cert install
```

## Verify

```sh
tokenmeter verify
# windsurf: OK
```

Or manually:

```sh
grep http.proxy "$HOME/Library/Application Support/Windsurf/User/settings.json"
```

## What's captured

| Traffic | How |
|---|---|
| Windsurf AI extension (Claude, GPT) | `ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL` → plain HTTP |
| Any HTTPS-tunnelled AI extension | `http.proxy` HTTPS CONNECT MITM |

## Settings file location

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/Windsurf/User/settings.json` |
| Linux | `~/.config/Windsurf/User/settings.json` |
| Windows | `%APPDATA%\Windsurf\User\settings.json` |

## Uninstall

```sh
tokenmeter uninstall   # reverts settings.json and shell env vars
```
