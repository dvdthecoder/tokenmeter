// Package bedrock implements ProviderPlugin for AWS Bedrock (Claude and other models
// served via the Bedrock Converse / InvokeModel APIs).
package bedrock

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

func init() {
	providers.Register(&Plugin{})
}

// Plugin handles traffic to *.bedrock.amazonaws.com and bedrock-runtime.*.amazonaws.com.
type Plugin struct{}

func (p *Plugin) Name() string { return "bedrock" }

func (p *Plugin) Detect(req *http.Request) bool {
	h := req.Host
	return strings.Contains(h, "bedrock") && strings.Contains(h, "amazonaws.com")
}

func (p *Plugin) UpstreamURL(req *http.Request, configuredBase string) string {
	if configuredBase != "" {
		return configuredBase
	}
	// Bedrock endpoints are regional; preserve the original host.
	return "https://" + req.Host
}

func (p *Plugin) ModifyRequest(req *http.Request) {}

// --- Non-streaming response ---

// Bedrock Converse API response (both Claude and other models).
type converseResponse struct {
	// Model ID is not in the response body — read from the URL path.
	Usage *struct {
		InputTokens  int64 `json:"inputTokens"`
		OutputTokens int64 `json:"outputTokens"`
	} `json:"usage"`
	// InvokeModelWithResponseStream uses the same shape for the MessageStart event.
}

func (p *Plugin) ParseResponse(body []byte) (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64, err error) {
	var r converseResponse
	if err = json.Unmarshal(body, &r); err != nil {
		return
	}
	if r.Usage != nil {
		input = r.Usage.InputTokens
		output = r.Usage.OutputTokens
	}
	return "", "", "", input, output, 0, 0, nil
}

// --- Streaming parser ---

func (p *Plugin) NewStreamParser() providers.StreamParser {
	return &streamParser{}
}

type streamParser struct {
	model  string
	input  int64
	output int64
}

// Bedrock streaming events are JSON lines wrapped in EventStream (binary framing
// handled by the SDK). Here we receive the inner JSON payloads.
type bedrockChunk struct {
	// InvokeModelWithResponseStream: metadata chunk
	AmazonBedrockInvocationMetrics *struct {
		InputTokenCount  int64 `json:"inputTokenCount"`
		OutputTokenCount int64 `json:"outputTokenCount"`
	} `json:"amazon-bedrock-invocationMetrics"`
	// Converse stream: metadata event
	Usage *struct {
		InputTokens  int64 `json:"inputTokens"`
		OutputTokens int64 `json:"outputTokens"`
	} `json:"usage"`
}

func (s *streamParser) ConsumeEvent(data []byte) error {
	var chunk bedrockChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil
	}
	if m := chunk.AmazonBedrockInvocationMetrics; m != nil {
		s.input = m.InputTokenCount
		s.output = m.OutputTokenCount
	}
	if u := chunk.Usage; u != nil {
		s.input = u.InputTokens
		s.output = u.OutputTokens
	}
	return nil
}

func (s *streamParser) Result() (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64) {
	return s.model, "", "", s.input, s.output, 0, 0
}

// --- Cost estimation ---
// Prices USD per million tokens (Bedrock on-demand pricing, us-east-1, mid-2025).

type modelPricing struct {
	inputPerM  float64
	outputPerM float64
}

var pricing = map[string]modelPricing{
	// Claude on Bedrock
	"anthropic.claude-opus-4-5":                  {15.00, 75.00},
	"anthropic.claude-sonnet-4-5":                {3.00, 15.00},
	"anthropic.claude-haiku-4-5":                 {0.80, 4.00},
	"anthropic.claude-3-5-sonnet-20241022":        {3.00, 15.00},
	"anthropic.claude-3-5-haiku-20241022":         {0.80, 4.00},
	"anthropic.claude-3-opus-20240229":            {15.00, 75.00},
	// Meta Llama on Bedrock
	"meta.llama3-70b-instruct-v1:0":              {2.65, 3.50},
	"meta.llama3-8b-instruct-v1:0":               {0.22, 0.22},
	// Mistral on Bedrock
	"mistral.mistral-large-2402-v1:0":            {4.00, 12.00},
	"mistral.mistral-7b-instruct-v0:2":           {0.15, 0.20},
	// Amazon Nova
	"amazon.nova-pro-v1:0":                       {0.80, 3.20},
	"amazon.nova-lite-v1:0":                      {0.06, 0.24},
	"amazon.nova-micro-v1:0":                     {0.035, 0.14},
}

func (p *Plugin) EstimateCost(model string, input, output, _, _ int64) float64 {
	pr, ok := pricing[model]
	if !ok {
		return 0
	}
	const M = 1_000_000.0
	return (float64(input)/M)*pr.inputPerM + (float64(output)/M)*pr.outputPerM
}
