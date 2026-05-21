# VS Code

VS Code itself doesn't call LLM APIs — its extensions do. tokenmeter supports three of the most popular:

| Extension | Hook | Auto-configured |
|---|---|---|
| Continue.dev | `OPENAI_BASE_URL` env var | ✅ via shell profile |
| Cline | `settings.json` patch | ✅ via `tokenmeter install` |
| GitHub Copilot | HTTPS MITM proxy | ✅ via `tokenmeter cert install` + `tokenmeter install` |

## Setup

```sh
tokenmeter install
tokenmeter verify
```

Then open a **new VS Code window** so it inherits the updated environment.

## Continue.dev

Continue reads `OPENAI_BASE_URL` from the environment. After `tokenmeter install` patches your shell profile, open a new VS Code window and Continue will route through tokenmeter automatically.

To confirm:

```sh
# Run a completion in Continue, then:
tokenmeter query --last 5m
```

## Cline

`tokenmeter install` automatically patches `~/.config/Code/User/settings.json` (Linux), `~/Library/Application Support/Code/User/settings.json` (macOS), or `%APPDATA%\Code\User\settings.json` (Windows) with:

```json
{
  "cline.apiProvider": "openai-compatible",
  "cline.openAiBaseUrl": "http://127.0.0.1:4191/v1"
}
```

Existing settings are preserved — only the Cline keys are added or updated.

To target VS Code only:

```sh
tokenmeter install --backend vscode
```

To revert:

```sh
tokenmeter uninstall   # removes cline.* keys from settings.json
```

## GitHub Copilot

GitHub Copilot hardcodes `api.githubcopilot.com` and does not honour `OPENAI_BASE_URL`. tokenmeter intercepts it via an **HTTPS MITM proxy** — a local CA signs per-host certificates on demand so the TLS handshake succeeds.

### One-time setup

**Step 1 — Generate and trust the local CA:**

```sh
tokenmeter cert install
```

This generates `~/.local/share/tokenmeter/ca.{key,crt}` and installs the certificate into your system trust store (macOS Keychain, Debian/Ubuntu `update-ca-certificates`, Fedora/Arch `trust`).

**Step 2 — Configure VS Code to proxy through tokenmeter:**

```sh
tokenmeter install   # or: tokenmeter install --backend vscode
```

This patches `settings.json` with:

```json
{
  "http.proxy": "http://127.0.0.1:4191",
  "http.proxyStrictSSL": false
}
```

**Step 3 — Reload VS Code:**

Open a new VS Code window. Copilot completions will now route through tokenmeter.

### Verify

```sh
# Trigger a Copilot completion, then:
tokenmeter query --last 5m
```

You should see events from provider `copilot` with model `gpt-4o` (or similar). Cost will be `$0.00` — Copilot is subscription-based.

### How it works

tokenmeter sits on port 4191. When VS Code sends `CONNECT api.githubcopilot.com:443` (the HTTPS proxy handshake), tokenmeter:

1. Accepts the tunnel and tells VS Code the connection is established
2. Generates a TLS certificate for `api.githubcopilot.com` signed by the local CA
3. Terminates TLS — the decrypted HTTP/1.1 request flows through the normal proxy pipeline
4. Forwards the request to the real `api.githubcopilot.com` over a fresh TLS connection

The CA private key never leaves your machine. Per-host certificates are cached in memory for the process lifetime.
