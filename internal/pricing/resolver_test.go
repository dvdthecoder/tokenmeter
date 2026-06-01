package pricing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testServer returns a test HTTP server that serves a minimal models.dev-style response.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	payload := map[string]any{
		"anthropic": map[string]any{
			"models": map[string]any{
				"claude-sonnet-4-6": map[string]any{
					"cost": map[string]any{
						"input":       3.0,
						"output":      15.0,
						"cache_read":  0.3,
						"cache_write": 3.75,
					},
				},
				"no-cost-model": map[string]any{
					// model with no cost field
				},
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
}

func newTestResolver(t *testing.T, srv *httptest.Server) *Resolver {
	t.Helper()
	dir := t.TempDir()
	r := &Resolver{
		cachePath: filepath.Join(dir, "pricing-cache.json"),
		client:    srv.Client(),
	}
	// Override the URL to point to the test server.
	// We inject the server URL via fetchRemote indirectly through a patched client.
	// Since modelsDevURL is a package-level const, we use a wrapper approach:
	// refresh → fetchRemote → GET modelsDevURL. For testing, we replace the client
	// with one that redirects to our test server using a custom RoundTripper.
	r.client = &http.Client{
		Transport: &redirectTransport{base: srv.URL},
	}
	return r
}

// redirectTransport rewrites any request URL host to the test server host.
type redirectTransport struct {
	base string
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Host = req.URL.Host
	// Parse base and override scheme+host
	baseURL := rt.base
	// Just replace the URL entirely with base + path
	clone.URL, _ = clone.URL.Parse(baseURL + req.URL.Path)
	return http.DefaultTransport.RoundTrip(clone)
}

func TestCostLookup(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	r := newTestResolver(t, srv)

	// 1M input tokens, 0 output/cache — cost = 3.0 USD
	cost := r.Cost("anthropic", "claude-sonnet-4-6", 1_000_000, 0, 0, 0)
	if cost != 3.0 {
		t.Errorf("input cost: got %.4f, want 3.0", cost)
	}
}

func TestCostOutputTokens(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	r := newTestResolver(t, srv)

	cost := r.Cost("anthropic", "claude-sonnet-4-6", 0, 1_000_000, 0, 0)
	if cost != 15.0 {
		t.Errorf("output cost: got %.4f, want 15.0", cost)
	}
}

func TestCostCacheRead(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	r := newTestResolver(t, srv)

	// 1M cached-read tokens: input=1M (total) of which 1M cached → regular=0, cached=1M
	cost := r.Cost("anthropic", "claude-sonnet-4-6", 1_000_000, 0, 1_000_000, 0)
	if cost != 0.3 {
		t.Errorf("cache_read cost: got %.4f, want 0.3", cost)
	}
}

func TestCostUnknownModel(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	r := newTestResolver(t, srv)

	cost := r.Cost("anthropic", "unknown-model-xyz", 1_000_000, 0, 0, 0)
	if cost != 0 {
		t.Errorf("unknown model: got %.4f, want 0", cost)
	}
}

func TestCostUnknownProvider(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	r := newTestResolver(t, srv)

	cost := r.Cost("unknownprovider", "some-model", 1_000_000, 0, 0, 0)
	if cost != 0 {
		t.Errorf("unknown provider: got %.4f, want 0", cost)
	}
}

func TestDiskCacheUsed(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	r := newTestResolver(t, srv)

	// First call populates the cache.
	_ = r.Cost("anthropic", "claude-sonnet-4-6", 1_000_000, 0, 0, 0)

	if _, err := os.Stat(r.cachePath); err != nil {
		t.Errorf("disk cache not written: %v", err)
	}

	// A new resolver with same path but no server should use disk cache.
	r2 := &Resolver{
		cachePath: r.cachePath,
		client:    &http.Client{}, // no working transport
	}
	// Pre-load disk cache manually.
	if err := r2.refresh(); err != nil {
		// If refresh fails because no server, try loadDiskCache directly.
		entry, err2 := r2.loadDiskCache()
		if err2 != nil {
			t.Fatalf("loadDiskCache: %v", err2)
		}
		r2.entry = entry
	}

	cost := r2.Cost("anthropic", "claude-sonnet-4-6", 1_000_000, 0, 0, 0)
	if cost != 3.0 {
		t.Errorf("disk cache cost: got %.4f, want 3.0", cost)
	}
}

func TestDiskCacheExpiry(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "pricing-cache.json")

	// Write a stale cache entry.
	stale := &cacheEntry{
		FetchedAt: time.Now().Add(-25 * time.Hour), // 25h ago → stale
		Providers: map[string]map[string]modelCost{
			"anthropic": {"stale-model": {Input: 999}},
		},
	}
	data, _ := json.Marshal(stale)
	os.WriteFile(cachePath, data, 0o600)

	srv := testServer(t)
	defer srv.Close()
	r := &Resolver{
		cachePath: cachePath,
		client: &http.Client{
			Transport: &redirectTransport{base: srv.URL},
		},
	}

	// Should refresh because cache is stale.
	cost := r.Cost("anthropic", "claude-sonnet-4-6", 1_000_000, 0, 0, 0)
	if cost != 3.0 {
		t.Errorf("after stale cache refresh: got %.4f, want 3.0", cost)
	}
}
