package proxy

import (
	"net/http"
	"testing"

	"github.com/dvdthecoder/tokenmeter/internal/config"
)

func newTestProxy() *Proxy {
	cfg := &config.Config{}
	cfg.Proxy.Upstreams = map[string]string{}
	return New(cfg)
}

// TestDirectorAnthropicPassthroughVersion verifies that requests carrying
// anthropic-version but not matching any provider are transparently forwarded
// to api.anthropic.com instead of returning a 502.
func TestDirectorAnthropicPassthroughVersion(t *testing.T) {
	p := newTestProxy()
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:4191/v1/models", nil)
	req.Header.Set("anthropic-version", "2023-06-01")

	p.director(req)

	if req.URL.Host != "api.anthropic.com" {
		t.Errorf("host: got %q, want api.anthropic.com", req.URL.Host)
	}
	if req.URL.Scheme != "https" {
		t.Errorf("scheme: got %q, want https", req.URL.Scheme)
	}
}

// TestDirectorAnthropicPassthroughAPIKey verifies the x-api-key fallback path.
func TestDirectorAnthropicPassthroughAPIKey(t *testing.T) {
	p := newTestProxy()
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:4191/v1/something", nil)
	req.Header.Set("x-api-key", "sk-ant-test")

	p.director(req)

	if req.URL.Host != "api.anthropic.com" {
		t.Errorf("host: got %q, want api.anthropic.com", req.URL.Host)
	}
}

// TestDirectorAnthropicPassthroughConfiguredUpstream verifies that a
// configured Anthropic upstream overrides the default api.anthropic.com.
func TestDirectorAnthropicPassthroughConfiguredUpstream(t *testing.T) {
	cfg := &config.Config{}
	cfg.Proxy.Upstreams = map[string]string{
		"anthropic": "https://custom.anthropic.example.com",
	}
	p := New(cfg)

	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:4191/v1/models", nil)
	req.Header.Set("anthropic-version", "2023-06-01")

	p.director(req)

	if req.URL.Host != "custom.anthropic.example.com" {
		t.Errorf("host: got %q, want custom.anthropic.example.com", req.URL.Host)
	}
}

// TestDirectorNoMatchNoAnthropicHeaders verifies that requests with no provider
// match and no anthropic-version/x-api-key are still forwarded to api.anthropic.com.
// This covers Claude Code OAuth token refresh (Authorization: Bearer claude_xxx)
// and /v1/models calls that carry no provider-identifying headers — dropping them
// breaks Claude Code auth.
func TestDirectorNoMatchNoAnthropicHeaders(t *testing.T) {
	p := newTestProxy()
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:4191/oauth/token", nil)
	req.Header.Set("Authorization", "Bearer claude_oauth_refresh_token")

	p.director(req)

	if req.URL.Host != "api.anthropic.com" {
		t.Errorf("host: got %q, want api.anthropic.com", req.URL.Host)
	}
	if req.URL.Scheme != "https" {
		t.Errorf("scheme: got %q, want https", req.URL.Scheme)
	}
}
