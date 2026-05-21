# Provider plugins

A provider plugin detects a specific LLM vendor and parses its wire format into a `UsageEvent`.

## Interface

```go
type ProviderPlugin interface {
    Name() string
    Detect(req *http.Request) bool
    UpstreamURL(req *http.Request, configuredBase string) string
    ModifyRequest(req *http.Request)
    ParseResponse(body []byte) (model, serviceTier, inferenceGeo string,
        input, output, cached, cachedCreation int64, err error)
    NewStreamParser() StreamParser
    EstimateCost(model string, input, output, cached, cachedCreation int64) float64
}

type StreamParser interface {
    ConsumeEvent(data []byte) error
    Result() (model, serviceTier, inferenceGeo string,
        input, output, cached, cachedCreation int64)
}
```

## Scaffold

```sh
tokenmeter scaffold provider gemini
# creates plugins/providers/gemini/gemini.go with stubs
```

## Reference: Anthropic provider

`plugins/providers/anthropic/anthropic.go` is the most complete example — it handles:

- `message_start` SSE event → input tokens, cache tiers, service tier, model
- `message_delta` SSE event → output tokens
- Non-streaming REST response
- Cost table with all Claude models including cache read (10%) and write (125%) pricing

## Reference: OpenAI provider

`plugins/providers/openai/openai.go` covers the OpenAI-compatible protocol used by vLLM, OpenCode, LiteLLM, and others:

- `ModifyRequest` injects `stream_options: {include_usage: true}` for streaming
- Final usage chunk carries `prompt_tokens`, `completion_tokens`, `prompt_tokens_details.cached_tokens`
- Unknown models return `$0.00` cost (correct for self-hosted)

## Reference: Gemini provider

`plugins/providers/gemini/gemini.go` handles the Google Gemini API (`generativelanguage.googleapis.com`):

- `ModifyRequest` is a no-op — Gemini streaming includes `usageMetadata` in the final chunk natively
- Stream parser overwrites on each chunk — the final chunk always has authoritative totals
- `usageMetadata.cachedContentTokenCount` mapped to `cached`; no cache-write token concept (always 0)
- Pricing table covers 7 models; cached tokens billed at 25% of input price; unknown models fall back to gemini-2.0-flash

| Model | Input / 1M | Output / 1M |
|---|---|---|
| gemini-2.5-pro | $1.25 | $10.00 |
| gemini-2.5-flash | $0.15 | $0.60 |
| gemini-2.0-flash | $0.10 | $0.40 |
| gemini-2.0-flash-lite | $0.075 | $0.30 |
| gemini-1.5-pro | $1.25 | $5.00 |
| gemini-1.5-flash | $0.075 | $0.30 |
| gemini-1.5-flash-8b | $0.0375 | $0.15 |

## Reference: Copilot provider

`plugins/providers/copilot/copilot.go` intercepts GitHub Copilot traffic routed through tokenmeter's MITM proxy:

- `Detect()` matches `api.githubcopilot.com` (with or without port)
- Wire format is OpenAI-compatible — stream parsing and response parsing delegate to the OpenAI plugin directly
- `EstimateCost()` always returns `0.0` — Copilot is subscription-based
- Requires the MITM CA to be installed (`tokenmeter cert install`) and VS Code proxied (`http.proxy` setting)

## Reference: Bedrock provider

`plugins/providers/bedrock/bedrock.go` handles AWS Bedrock's two invocation APIs:

- `Detect()` matches any host containing `bedrock` and `amazonaws.com`
- **Converse API** (`/converse`, `/converse-stream`): parses `usage.inputTokens` / `usage.outputTokens`
- **InvokeModelWithResponseStream**: parses the `amazon-bedrock-invocationMetrics` metadata event
- `UpstreamURL()` preserves the original regional host (e.g. `bedrock-runtime.us-east-1.amazonaws.com`) — SigV4 signing is done by the calling SDK, not tokenmeter
- Cost table covers Claude, Llama, Mistral, and Amazon Nova models on Bedrock (us-east-1, on-demand)

| Model | Input / 1M | Output / 1M |
|---|---|---|
| anthropic.claude-opus-4-5 | $15.00 | $75.00 |
| anthropic.claude-sonnet-4-5 | $3.00 | $15.00 |
| anthropic.claude-haiku-4-5 | $0.80 | $4.00 |
| anthropic.claude-3-5-sonnet-20241022 | $3.00 | $15.00 |
| meta.llama3-70b-instruct-v1:0 | $2.65 | $3.50 |
| amazon.nova-pro-v1:0 | $0.80 | $3.20 |
| amazon.nova-lite-v1:0 | $0.06 | $0.24 |

## Writing a new provider

1. Run `tokenmeter scaffold provider <name>`
2. Implement `Detect()` — return true for requests that belong to this vendor
3. Implement `NewStreamParser()` and `ConsumeEvent()` — feed SSE data lines
4. Implement `ParseResponse()` — parse the full non-streaming body
5. Implement `EstimateCost()` — return USD cost, `0` if unknown
6. Add blank import in `cmd/tokenmeter/main.go`
7. Write unit tests with fixture response bodies (no live network calls)

## Testing

```go
// Use inline fixture strings — no network calls needed
var streamFixture = []string{
    `{"type":"message_start","message":{"model":"...", "usage":{...}}}`,
    `{"type":"message_delta","usage":{"output_tokens":75}}`,
}

func TestStreamParser(t *testing.T) {
    p := &Plugin{}
    sp := p.NewStreamParser()
    for _, line := range streamFixture {
        sp.ConsumeEvent([]byte(line))
    }
    model, _, _, input, output, cached, creation := sp.Result()
    // assert ...
}
```
