# Known Issues & Fixes

Major problems encountered in production use, with root causes and the fixes shipped.

---

## GH#17 — Proxy returns 502 for unmatched routes

**Severity:** High  
**Fixed in:** commit `fdad7f3`

### Symptom

After running `tokenmeter install`, Claude Code CLI would silently break. Every request returned 502 Bad Gateway. The Claude CLI also sends a `HEAD /` pre-flight connectivity check on startup; that 502 was enough for it to refuse to route through the proxy at all.

Any Anthropic endpoint that doesn't carry an `anthropic-version` header — OAuth callbacks, model listing, anything outside the messages path — also returned 502.

### Root cause

`director()` in `internal/proxy/proxy.go` calls `providers.Detect(req)` and if no provider matches, returns without setting `req.URL.Scheme` or `req.URL.Host`. `httputil.ReverseProxy` then attempts to forward to an incomplete URL, which always fails with 502.

### Fix

Two targeted changes:

1. **`HEAD /` short-circuit** — added an explicit handler in `cmd/tokenmeter/main.go` that returns `200 OK` before the request ever reaches the proxy:

    ```go
    mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodHead && r.URL.Path == "/" {
            w.WriteHeader(http.StatusOK)
            return
        }
        p.ServeHTTP(w, r)
    }))
    ```

2. **Anthropic passthrough fallback** — in `director()`, requests that carry `anthropic-version` or `x-api-key` headers but don't match a registered provider are forwarded transparently to `api.anthropic.com` (or the configured upstream):

    ```go
    if req.Header.Get("anthropic-version") != "" || req.Header.Get("x-api-key") != "" {
        base := p.cfg.Proxy.Upstreams["anthropic"]
        if base == "" {
            base = "https://api.anthropic.com"
        }
        upstream, _ := url.Parse(base)
        req.URL.Scheme = upstream.Scheme
        req.URL.Host = upstream.Host
        req.Host = upstream.Host
        return
    }
    ```

No usage is captured on the passthrough path — it's transparent forwarding only.

---

## GH#18 — MITM streaming buffered entire SSE response before forwarding

**Severity:** Medium  
**Fixed in:** commit `fdad7f3`

### Symptom

GitHub Copilot and AWS Bedrock requests routed through the HTTPS CONNECT MITM tunnel appeared to hang until the full response was received, then delivered all at once. Streaming completions felt identical to non-streaming ones — no incremental tokens, no perceived responsiveness.

### Root cause

The `responseWriter` in `internal/mitm/connect.go` accumulated the entire response body in a `[]byte` slice and only wrote it to the TLS connection inside `flush()`, after `httputil.ReverseProxy` had finished. It also didn't implement `http.Flusher`, so the reverse proxy had no way to push chunks to the wire as they arrived.

### Fix

`responseWriter` now detects `text/event-stream` in `WriteHeader` and switches to a streaming mode:

- **Headers** are written immediately to a `bufio.Writer` backed by the TLS connection, using `Transfer-Encoding: chunked` (incompatible with `Content-Length`, which is dropped).
- **Each `Write()` call** is emitted as an HTTP/1.1 chunked frame (`<hex-len>\r\n<data>\r\n`).
- **`Flush()`** (implementing `http.Flusher`) flushes the `bufio.Writer` buffer so backpressure from `httputil.ReverseProxy` is honoured.
- **`flush()`** (the connection finaliser) writes the terminal chunk (`0\r\n\r\n`) to signal end of stream.

Non-streaming responses (`application/json`, etc.) continue to use the original buffered path unchanged.

The struct field `conn *tls.Conn` was relaxed to `conn io.Writer` — `*tls.Conn` satisfies `io.Writer`, so no behaviour change in production — but it makes the streaming path unit-testable with a plain `bytes.Buffer`.

---

## How to report

If you hit something not listed here, [open a bug report](https://github.com/dvdthecoder/tokenmeter/issues/new?template=bug_report.md). Include:

- tokenmeter version (`tokenmeter version`)
- AI tool and version
- Provider (Anthropic / OpenAI / Copilot / Bedrock / Gemini)
- Whether streaming was enabled
- Relevant log lines (`tokenmeter daemon` runs with `--log-level debug` for verbose output)
