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

// TestDirectorNoMatchNoAnthropicHeaders verifies that requests without
// Anthropic headers and no matching provider leave the URL unmodified
// (the reverse proxy will error, which is intentional).
func TestDirectorNoMatchNoAnthropicHeaders(t *testing.T) {
	p := newTestProxy()
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:4191/some/random/path", nil)
	originalHost := req.URL.Host

	p.director(req)

	// URL host is unchanged — proxy will 502, which is correct for unknown routes.
	if req.URL.Host != originalHost {
		t.Errorf("host changed unexpectedly: %q → %q", originalHost, req.URL.Host)
	}
}
