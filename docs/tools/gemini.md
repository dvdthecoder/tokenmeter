# Gemini CLI

The Google Gemini CLI (`gemini`) and any client using the Google AI SDK connect to `generativelanguage.googleapis.com` over HTTPS. tokenmeter intercepts this traffic via the same **HTTPS MITM proxy** it uses for GitHub Copilot — a locally-trusted CA signs per-host certificates on demand.

!!! info "No env-var hook"
    Unlike Claude Code and Codex, there is no `GEMINI_BASE_URL` env var that re-points the CLI to tokenmeter. Routing works via `HTTPS_PROXY` instead.

## Setup

### Step 1 — Generate and trust the local CA

```sh
tokenmeter cert install
```

This generates `~/.local/share/tokenmeter/ca.{key,crt}` and installs the certificate into your system trust store (macOS Keychain, Debian/Ubuntu `update-ca-certificates`, Fedora/Arch `trust`).

### Step 2 — Set `HTTPS_PROXY` in your shell profile

Add to `~/.zshrc`, `~/.bashrc`, or equivalent:

```sh
export HTTPS_PROXY=http://127.0.0.1:4191
```

Then reload:

```sh
source ~/.zshrc   # or ~/.bashrc
```

### Step 3 — Verify

```sh
gemini "what is 2+2"
tokenmeter query --last 5m
```

You should see events with `provider=gemini` and a model like `gemini-2.0-flash`.

---

## What's captured

| Field | Source |
|---|---|
| `TokensInput` | `usageMetadata.promptTokenCount` (final SSE chunk) |
| `TokensOutput` | `usageMetadata.candidatesTokenCount` (final SSE chunk) |
| `TokensCached` | `usageMetadata.cachedContentTokenCount` |
| `CostUSD` | Estimated from model pricing table (see below) |
| `Model` | `modelVersion` field |

Gemini does not have cache-write tokens — `TokensCachedCreation` is always `0`.

For streaming, tokenmeter buffers the last ~2 KB of the response to extract `usageMetadata` from the final SSE chunk. Intermediate chunks may carry partial counts; the final chunk is always authoritative.

---

## Cost estimation

Prices are applied per million tokens (mid-2026):

| Model | Input / 1M | Output / 1M | Cached / 1M |
|---|---|---|---|
| `gemini-2.5-pro` | $1.25 | $10.00 | $0.3125 |
| `gemini-2.5-flash` | $0.15 | $0.60 | $0.0375 |
| `gemini-2.0-flash` | $0.10 | $0.40 | $0.025 |
| `gemini-2.0-flash-lite` | $0.075 | $0.30 | $0.01875 |
| `gemini-1.5-pro` | $1.25 | $5.00 | $0.3125 |
| `gemini-1.5-flash` | $0.075 | $0.30 | $0.01875 |
| `gemini-1.5-flash-8b` | $0.0375 | $0.15 | $0.009375 |

Cached tokens are billed at 25% of the input price. Unknown model names fall back to `gemini-2.0-flash` pricing.

---

## How MITM interception works

When a Gemini client sends a request with `HTTPS_PROXY` set:

1. The client sends `CONNECT generativelanguage.googleapis.com:443` to tokenmeter
2. tokenmeter accepts the tunnel, generates a TLS certificate for `generativelanguage.googleapis.com` signed by the local CA, and terminates TLS
3. The decrypted HTTP/1.1 request flows through the normal proxy pipeline — the Gemini provider plugin detects it via the `generativelanguage.googleapis.com` host
4. tokenmeter forwards the request to the real `generativelanguage.googleapis.com` over a fresh TLS connection and streams the response back

The CA private key never leaves your machine. Per-host certificates are cached in memory for the process lifetime.

---

## Uninstall

Remove the `HTTPS_PROXY` line from your shell profile, then:

```sh
tokenmeter uninstall   # stops daemon, removes env-var block
```

The CA certificate stays in your trust store — remove it manually via Keychain Access (macOS) or `update-ca-certificates` (Linux) if desired.
