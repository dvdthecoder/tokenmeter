// Package anthropic implements ProviderPlugin for the Anthropic Claude API.
package anthropic

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

func init() {
	providers.Register(&Plugin{})
}

type Plugin struct{}

func (p *Plugin) Name() string { return "anthropic" }

func (p *Plugin) Detect(req *http.Request) bool {
	return strings.Contains(req.Host, "anthropic.com") ||
		req.Header.Get("anthropic-version") != "" ||
		(req.Header.Get("x-api-key") != "" && strings.Contains(req.URL.Path, "/messages"))
}

func (p *Plugin) UpstreamURL(req *http.Request, configuredBase string) string {
	if configuredBase != "" {
		return configuredBase
	}
	return "https://api.anthropic.com"
}

func (p *Plugin) ModifyRequest(_ *http.Request) {}

// --- Non-streaming response ---

type anthropicResponse struct {
	Model string `json:"model"`
	Usage struct {
		InputTokens              int64  `json:"input_tokens"`
		OutputTokens             int64  `json:"output_tokens"`
		CacheReadInputTokens     int64  `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64  `json:"cache_creation_input_tokens"`
		ServiceTier              string `json:"service_tier"`
		InferenceGeo             string `json:"inference_geo"`
	} `json:"usage"`
}

func (p *Plugin) ParseResponse(body []byte) (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64, err error) {
	var r anthropicResponse
	if err = json.Unmarshal(body, &r); err != nil {
		return
	}
	return r.Model,
		r.Usage.ServiceTier,
		r.Usage.InferenceGeo,
		r.Usage.InputTokens,
		r.Usage.OutputTokens,
		r.Usage.CacheReadInputTokens,
		r.Usage.CacheCreationInputTokens,
		nil
}

// --- Streaming parser ---

func (p *Plugin) NewStreamParser() providers.StreamParser {
	return &streamParser{}
}

type streamParser struct {
	model          string
	serviceTier    string
	inferenceGeo   string
	input          int64
	output         int64
	cached         int64
	cachedCreation int64
}

type eventType struct {
	Type string `json:"type"`
}

// message_start carries input tokens and model.
type msgStartEvent struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int64  `json:"input_tokens"`
			CacheReadInputTokens     int64  `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64  `json:"cache_creation_input_tokens"`
			ServiceTier              string `json:"service_tier"`
			InferenceGeo             string `json:"inference_geo"`
			CacheCreation            struct {
				Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens"`
				Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	} `json:"message"`
}

// message_delta carries output tokens (and may repeat input for completeness).
type msgDeltaEvent struct {
	Type  string `json:"type"`
	Usage struct {
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

func (s *streamParser) ConsumeEvent(data []byte) error {
	var et eventType
	if err := json.Unmarshal(data, &et); err != nil {
		return nil
	}
	switch et.Type {
	case "message_start":
		var e msgStartEvent
		if err := json.Unmarshal(data, &e); err == nil {
			s.model = e.Message.Model
			s.input = e.Message.Usage.InputTokens
			s.cached = e.Message.Usage.CacheReadInputTokens
			// Sum both ephemeral cache creation tiers.
			s.cachedCreation = e.Message.Usage.CacheCreationInputTokens +
				e.Message.Usage.CacheCreation.Ephemeral5mInputTokens +
				e.Message.Usage.CacheCreation.Ephemeral1hInputTokens
			s.serviceTier = e.Message.Usage.ServiceTier
			s.inferenceGeo = e.Message.Usage.InferenceGeo
		}
	case "message_delta":
		var e msgDeltaEvent
		if err := json.Unmarshal(data, &e); err == nil {
			s.output = e.Usage.OutputTokens
		}
	}
	return nil
}

func (s *streamParser) Result() (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64) {
	return s.model, s.serviceTier, s.inferenceGeo,
		s.input, s.output, s.cached, s.cachedCreation
}

// --- Cost estimation (USD per million tokens, mid-2025) ---

type modelPricing struct {
	inputPerM        float64
	outputPerM       float64
	cacheReadPerM    float64
	cacheWritePerM   float64
}

var pricing = map[string]modelPricing{
	"claude-opus-4-7":           {15.00, 75.00, 1.50, 3.75},
	"claude-opus-4-5":           {15.00, 75.00, 1.50, 3.75},
	"claude-sonnet-4-6":         {3.00, 15.00, 0.30, 0.75},
	"claude-sonnet-4-5":         {3.00, 15.00, 0.30, 0.75},
	"claude-haiku-4-5-20251001": {0.80, 4.00, 0.08, 0.20},
}

func (p *Plugin) EstimateCost(model string, input, output, cached, cachedCreation int64) float64 {
	pr, ok := pricing[model]
	if !ok {
		pr = pricing["claude-sonnet-4-6"]
	}
	const M = 1_000_000.0
	return (float64(input)/M)*pr.inputPerM +
		(float64(output)/M)*pr.outputPerM +
		(float64(cached)/M)*pr.cacheReadPerM +
		(float64(cachedCreation)/M)*pr.cacheWritePerM
}
