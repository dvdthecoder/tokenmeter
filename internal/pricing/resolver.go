// Package pricing resolves per-model LLM costs, with an optional remote
// fallback to models.dev for models not priced by the built-in provider plugins.
package pricing

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	modelsDevURL  = "https://models.dev/api.json"
	cacheTTL      = 24 * time.Hour
	fetchTimeout  = 10 * time.Second
)

// modelCost holds per-million-token prices for a single model.
type modelCost struct {
	Input       float64 `json:"input"`
	Output      float64 `json:"output"`
	CacheRead   float64 `json:"cache_read"`
	CacheWrite  float64 `json:"cache_write"`
}

// cacheEntry is the on-disk cache format.
type cacheEntry struct {
	FetchedAt time.Time                       `json:"fetched_at"`
	Providers map[string]map[string]modelCost `json:"providers"`
}

// Resolver fetches and caches pricing data from models.dev.
// Cost is returned in USD for the given token counts.
type Resolver struct {
	mu        sync.RWMutex
	cachePath string
	client    *http.Client
	entry     *cacheEntry // in-memory cache
}

// New creates a Resolver with the given cache path. If cachePath is empty the
// default ~/.local/share/tokenmeter/pricing-cache.json is used.
func New(cachePath string) *Resolver {
	if cachePath == "" {
		home, _ := os.UserHomeDir()
		cachePath = filepath.Join(home, ".local", "share", "tokenmeter", "pricing-cache.json")
	}
	return &Resolver{
		cachePath: cachePath,
		client:    &http.Client{Timeout: fetchTimeout},
	}
}

// Cost returns the estimated cost in USD for a request. Returns 0 if pricing is
// unknown for the given provider+model combination.
// input/output/cachedRead/cachedCreation are in tokens; prices are per million tokens.
func (r *Resolver) Cost(provider, model string, input, output, cachedRead, cachedCreation int64) float64 {
	prices, ok := r.lookup(provider, model)
	if !ok {
		return 0
	}
	// cachedRead tokens are billed at cache_read rate (not input rate).
	// Regular input = total input - cachedRead - cachedCreation tokens.
	regularInput := input - cachedRead - cachedCreation
	if regularInput < 0 {
		regularInput = 0
	}
	cost := float64(regularInput)*prices.Input/1e6 +
		float64(output)*prices.Output/1e6 +
		float64(cachedRead)*prices.CacheRead/1e6 +
		float64(cachedCreation)*prices.CacheWrite/1e6
	return cost
}

// lookup returns the modelCost for the given provider+model, refreshing the
// cache if it is stale or missing.
func (r *Resolver) lookup(provider, model string) (modelCost, bool) {
	r.mu.RLock()
	entry := r.entry
	r.mu.RUnlock()

	if entry == nil || time.Since(entry.FetchedAt) > cacheTTL {
		if err := r.refresh(); err != nil {
			slog.Warn("pricing cache refresh failed", "err", err)
			if entry == nil {
				return modelCost{}, false
			}
		}
		r.mu.RLock()
		entry = r.entry
		r.mu.RUnlock()
	}

	if entry == nil {
		return modelCost{}, false
	}
	models, ok := entry.Providers[provider]
	if !ok {
		return modelCost{}, false
	}
	mc, ok := models[model]
	return mc, ok
}

// refresh loads cache from disk (if fresh) or fetches from models.dev.
func (r *Resolver) refresh() error {
	// Try disk cache first.
	if entry, err := r.loadDiskCache(); err == nil && time.Since(entry.FetchedAt) <= cacheTTL {
		r.mu.Lock()
		r.entry = entry
		r.mu.Unlock()
		return nil
	}

	// Fetch fresh data.
	entry, err := r.fetchRemote()
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.entry = entry
	r.mu.Unlock()

	if err := r.saveDiskCache(entry); err != nil {
		slog.Warn("pricing: failed to write disk cache", "path", r.cachePath, "err", err)
	}
	slog.Info("pricing: fetched remote model prices", "providers", len(entry.Providers))
	return nil
}

// fetchRemote downloads the models.dev API and parses it into a cacheEntry.
func (r *Resolver) fetchRemote() (*cacheEntry, error) {
	resp, err := r.client.Get(modelsDevURL)
	if err != nil {
		return nil, fmt.Errorf("pricing fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pricing fetch: HTTP %d", resp.StatusCode)
	}

	// models.dev format: map[providerID]{ models: map[modelID]{ cost: {...} } }
	var raw map[string]struct {
		Models map[string]struct {
			Cost *modelCost `json:"cost"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("pricing parse: %w", err)
	}

	entry := &cacheEntry{
		FetchedAt: time.Now(),
		Providers: make(map[string]map[string]modelCost, len(raw)),
	}
	for prov, pData := range raw {
		if len(pData.Models) == 0 {
			continue
		}
		mmap := make(map[string]modelCost, len(pData.Models))
		for modelID, mData := range pData.Models {
			if mData.Cost != nil {
				mmap[modelID] = *mData.Cost
			}
		}
		if len(mmap) > 0 {
			entry.Providers[prov] = mmap
		}
	}
	return entry, nil
}

func (r *Resolver) loadDiskCache() (*cacheEntry, error) {
	data, err := os.ReadFile(r.cachePath)
	if err != nil {
		return nil, err
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *Resolver) saveDiskCache(entry *cacheEntry) error {
	if err := os.MkdirAll(filepath.Dir(r.cachePath), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(r.cachePath, data, 0o600)
}
