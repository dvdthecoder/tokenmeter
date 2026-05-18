// Package providers defines the ProviderPlugin interface.
// Each provider plugin detects and parses a specific LLM vendor's wire format
// into a normalised UsageEvent. Plugins self-register via Register() in init().
package providers

import (
	"net/http"
	"time"
)

// UsageEvent is the normalised token usage record emitted after every proxied request.
// It contains only metadata — never prompt or response content.
type UsageEvent struct {
	// Identity
	RequestID string
	SessionID string // optional: groups requests in one conversation
	ServiceID string // hashed at storage layer for GDPR

	// Attribution
	Username      string // system user or TOKENMETER_USER env var
	ClientName    string // "claude-code-cli", "claude-code-app", "cursor", "codex-cli", ...
	ClientVersion string // parsed from User-Agent

	// Provider
	Provider     string // "anthropic", "openai"
	Model        string
	ServiceTier  string // "standard", "priority" — provider-specific
	InferenceGeo string // region where inference ran — provider-specific

	// Token counts
	TokensInput          int64
	TokensOutput         int64
	TokensCached         int64  // cache-read tokens (cheaper)
	TokensCachedCreation int64  // cache-write tokens (one-time cost)

	// Performance & cost
	LatencyMS     int64
	CostUSD       float64
	Timestamp     time.Time
	StreamingMode bool
}

// StreamParser processes an SSE stream incrementally.
// One instance is created per request via NewStreamParser().
type StreamParser interface {
	// ConsumeEvent is called for each SSE data payload (the bytes after "data: ").
	// Must not retain the slice after returning.
	ConsumeEvent(data []byte) error

	// Result returns the accumulated values after the stream ends.
	Result() (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64)
}

// ProviderPlugin detects and parses a vendor's HTTP wire format.
// Register implementations via Register() in an init() function.
type ProviderPlugin interface {
	// Name returns the canonical provider name, e.g. "anthropic".
	Name() string

	// Detect returns true if this plugin should handle the given outbound request.
	Detect(req *http.Request) bool

	// UpstreamURL returns the real upstream base URL for this provider.
	UpstreamURL(req *http.Request, configuredBase string) string

	// ModifyRequest mutates the outbound request before forwarding.
	ModifyRequest(req *http.Request)

	// ParseResponse extracts fields from a complete non-streaming response body.
	ParseResponse(body []byte) (model, serviceTier, inferenceGeo string, input, output, cached, cachedCreation int64, err error)

	// NewStreamParser returns a fresh StreamParser for one streaming request.
	NewStreamParser() StreamParser

	// EstimateCost returns the USD cost for the given token counts and model.
	EstimateCost(model string, input, output, cached, cachedCreation int64) float64
}

var registry = map[string]ProviderPlugin{}

func Register(p ProviderPlugin)              { registry[p.Name()] = p }
func All() map[string]ProviderPlugin         { return registry }
func Detect(req *http.Request) (ProviderPlugin, bool) {
	for _, p := range registry {
		if p.Detect(req) {
			return p, true
		}
	}
	return nil, false
}
