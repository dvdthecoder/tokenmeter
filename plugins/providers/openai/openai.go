// Package openai implements ProviderPlugin for OpenAI and OpenAI-compatible APIs
// (vLLM, OpenCode runner, Codex CLI, LiteLLM, Ollama, etc.).
package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/yourorg/tokenmeter/plugins/providers"
)

func init() {
	providers.Register(&Plugin{})
}

// Plugin handles api.openai.com and any host that speaks the OpenAI Chat Completions protocol.
// It intentionally matches broadly so vLLM and other compatible endpoints are covered.
type Plugin struct{}

func (p *Plugin) Name() string { return "openai" }

func (p *Plugin) Detect(req *http.Request) bool {
	// Explicit OpenAI host.
	if strings.Contains(req.Host, "openai.com") {
		return true
	}
	// OpenAI-compatible: Authorization: Bearer ... + /v1/chat/completions path.
	return strings.Contains(req.URL.Path, "/chat/completions") &&
		strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ")
}

func (p *Plugin) UpstreamURL(req *http.Request, configuredBase string) string {
	if configuredBase != "" {
		return configuredBase
	}
	return "https://api.openai.com"
}

// ModifyRequest injects stream_options.include_usage = true so that the final
// SSE chunk carries the usage object. Without this, usage is absent from streams.
func (p *Plugin) ModifyRequest(req *http.Request) {
	if req.Body == nil || req.ContentLength == 0 {
		return
	}
	// Only modify streaming requests.
	body, err := readAndRestore(req)
	if err != nil || len(body) == 0 {
		return
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return
	}

	// Check if stream: true
	var isStream bool
	if raw, ok := obj["stream"]; ok {
		_ = json.Unmarshal(raw, &isStream)
	}
	if !isStream {
		return
	}

	// Inject stream_options if not already present.
	if _, ok := obj["stream_options"]; !ok {
		obj["stream_options"] = json.RawMessage(`{"include_usage":true}`)
		modified, err := json.Marshal(obj)
		if err == nil {
			req.Body = io.NopCloser(bytes.NewReader(modified))
			req.ContentLength = int64(len(modified))
		}
	}
}

// --- Non-streaming response ---

type openAIResponse struct {
	Model string `json:"model"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

func (p *Plugin) ParseResponse(body []byte) (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64, err error) {
	var r openAIResponse
	if err = json.Unmarshal(body, &r); err != nil {
		return
	}
	if r.Usage.PromptTokensDetails != nil {
		cached = r.Usage.PromptTokensDetails.CachedTokens
	}
	return r.Model, "", "", r.Usage.PromptTokens, r.Usage.CompletionTokens, cached, 0, nil
}

// --- Streaming parser ---

func (p *Plugin) NewStreamParser() providers.StreamParser {
	return &streamParser{}
}

type streamParser struct {
	model  string
	input  int64
	output int64
	cached int64
}

// OpenAI streams the usage in the second-to-last chunk (before [DONE])
// when stream_options.include_usage is true.
type streamChunk struct {
	Model   string `json:"model"`
	Choices []struct{} `json:"choices"`
	Usage   *struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

func (s *streamParser) ConsumeEvent(data []byte) error {
	var chunk streamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil
	}
	if chunk.Model != "" {
		s.model = chunk.Model
	}
	if chunk.Usage != nil {
		s.input = chunk.Usage.PromptTokens
		s.output = chunk.Usage.CompletionTokens
		if chunk.Usage.PromptTokensDetails != nil {
			s.cached = chunk.Usage.PromptTokensDetails.CachedTokens
		}
	}
	return nil
}

func (s *streamParser) Result() (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64) {
	return s.model, "", "", s.input, s.output, s.cached, 0
}

// --- Cost estimation ---
// Prices in USD per million tokens (as of mid-2025).

type modelPricing struct {
	inputPerM  float64
	outputPerM float64
	cachedPerM float64
}

var pricing = map[string]modelPricing{
	"gpt-4o":              {2.50, 10.00, 1.25},
	"gpt-4o-mini":         {0.15, 0.60, 0.075},
	"gpt-4-turbo":         {10.00, 30.00, 5.00},
	"gpt-4":               {30.00, 60.00, 15.00},
	"gpt-3.5-turbo":       {0.50, 1.50, 0.25},
	// vLLM self-hosted — no direct cost; set to 0 so CostUSD reflects infra cost elsewhere.
	"qwen2.5-coder-7b":    {0, 0, 0},
	"qwen2.5-coder-32b":   {0, 0, 0},
	"qwen3-coder":         {0, 0, 0},
}

func (p *Plugin) EstimateCost(model string, input, output, cached, _ int64) float64 {
	pr, ok := pricing[model]
	if !ok {
		return 0 // unknown / self-hosted model
	}
	const M = 1_000_000.0
	return (float64(input)/M)*pr.inputPerM +
		(float64(output)/M)*pr.outputPerM +
		(float64(cached)/M)*pr.cachedPerM
}

// readAndRestore reads req.Body and replaces it so it can be read again.
func readAndRestore(req *http.Request) ([]byte, error) {
	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, err
}
