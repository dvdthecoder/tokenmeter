// Package gemini implements ProviderPlugin for the Google Gemini API
// (generativelanguage.googleapis.com).
package gemini

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

func (p *Plugin) Name() string { return "gemini" }

func (p *Plugin) Detect(req *http.Request) bool {
	return strings.Contains(req.Host, "generativelanguage.googleapis.com")
}

func (p *Plugin) UpstreamURL(_ *http.Request, configuredBase string) string {
	if configuredBase != "" {
		return configuredBase
	}
	return "https://generativelanguage.googleapis.com"
}

// ModifyRequest is a no-op — Gemini streaming includes usageMetadata in the
// final chunk without needing request-side injection.
func (p *Plugin) ModifyRequest(_ *http.Request) {}

// --- Non-streaming response ---

type geminiResponse struct {
	ModelVersion  string `json:"modelVersion"`
	UsageMetadata struct {
		PromptTokenCount     int64 `json:"promptTokenCount"`
		CandidatesTokenCount int64 `json:"candidatesTokenCount"`
		CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
}

func (p *Plugin) ParseResponse(body []byte) (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64, err error) {
	var r geminiResponse
	if err = json.Unmarshal(body, &r); err != nil {
		return
	}
	return r.ModelVersion, "", "",
		r.UsageMetadata.PromptTokenCount,
		r.UsageMetadata.CandidatesTokenCount,
		r.UsageMetadata.CachedContentTokenCount,
		0, nil
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

// geminiChunk represents one SSE data payload. Each streaming chunk is a
// complete JSON object; usageMetadata in the last chunk holds final counts.
type geminiChunk struct {
	ModelVersion  string `json:"modelVersion"`
	UsageMetadata *struct {
		PromptTokenCount        int64 `json:"promptTokenCount"`
		CandidatesTokenCount    int64 `json:"candidatesTokenCount"`
		CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
}

func (s *streamParser) ConsumeEvent(data []byte) error {
	var c geminiChunk
	if err := json.Unmarshal(data, &c); err != nil {
		return nil // skip malformed chunks
	}
	if c.ModelVersion != "" {
		s.model = c.ModelVersion
	}
	if c.UsageMetadata != nil {
		// Always overwrite — the final chunk has the authoritative totals.
		s.input = c.UsageMetadata.PromptTokenCount
		s.output = c.UsageMetadata.CandidatesTokenCount
		s.cached = c.UsageMetadata.CachedContentTokenCount
	}
	return nil
}

func (s *streamParser) Result() (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64) {
	return s.model, "", "", s.input, s.output, s.cached, 0
}

// --- Cost estimation (USD per million tokens, mid-2026) ---

type modelPricing struct{ inputPerM, outputPerM float64 }

var pricing = map[string]modelPricing{
	"gemini-2.5-pro":         {1.25, 10.00},
	"gemini-2.5-flash":       {0.15, 0.60},
	"gemini-2.0-flash":       {0.10, 0.40},
	"gemini-2.0-flash-lite":  {0.075, 0.30},
	"gemini-1.5-pro":         {1.25, 5.00},
	"gemini-1.5-flash":       {0.075, 0.30},
	"gemini-1.5-flash-8b":    {0.0375, 0.15},
}

func (p *Plugin) EstimateCost(model string, input, output, cached, _ int64) float64 {
	pr, ok := pricing[model]
	if !ok {
		// Unknown Gemini model — use flash pricing as a conservative estimate.
		pr = pricing["gemini-2.0-flash"]
	}
	const M = 1_000_000.0
	// Cached tokens billed at 25% of input price on Gemini.
	return (float64(input)/M)*pr.inputPerM +
		(float64(output)/M)*pr.outputPerM +
		(float64(cached)/M)*(pr.inputPerM*0.25)
}
