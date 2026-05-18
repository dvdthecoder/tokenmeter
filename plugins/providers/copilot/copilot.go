// Package copilot implements ProviderPlugin for the GitHub Copilot API
// (api.githubcopilot.com). The wire format is OpenAI-compatible chat completions.
package copilot

import (
	"net/http"
	"strings"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
	"github.com/dvdthecoder/tokenmeter/plugins/providers/openai"
)

func init() {
	providers.Register(&Plugin{})
}

// Plugin handles GitHub Copilot traffic.
// Copilot uses OpenAI-compatible chat completions — parsing is delegated to the
// OpenAI provider. Detection must run before the generic OpenAI plugin so Copilot
// events carry provider="copilot" rather than provider="openai".
type Plugin struct {
	oai *openai.Plugin
}

func (p *Plugin) Name() string { return "copilot" }

func (p *Plugin) Detect(req *http.Request) bool {
	return strings.Contains(req.Host, "api.githubcopilot.com")
}

func (p *Plugin) UpstreamURL(_ *http.Request, configuredBase string) string {
	if configuredBase != "" {
		return configuredBase
	}
	return "https://api.githubcopilot.com"
}

// ModifyRequest injects stream_options.include_usage just like the OpenAI plugin.
func (p *Plugin) ModifyRequest(req *http.Request) {
	p.delegate().ModifyRequest(req)
}

func (p *Plugin) ParseResponse(body []byte) (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64, err error) {
	return p.delegate().ParseResponse(body)
}

func (p *Plugin) NewStreamParser() providers.StreamParser {
	return p.delegate().NewStreamParser()
}

// EstimateCost returns 0 — Copilot is a flat subscription, not billed per token.
func (p *Plugin) EstimateCost(_ string, _, _, _, _ int64) float64 {
	return 0.0
}

func (p *Plugin) delegate() *openai.Plugin {
	if p.oai == nil {
		p.oai = &openai.Plugin{}
	}
	return p.oai
}
