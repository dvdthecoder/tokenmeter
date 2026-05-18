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
