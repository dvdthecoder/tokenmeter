// Package costalert is a MiddlewarePlugin that fires a warning (and optionally
// a webhook) when a single request exceeds a configured USD cost threshold.
package costalert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dvdthecoder/tokenmeter/plugins/middleware"
	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

func init() {
	middleware.Register(&Plugin{})
}

// Plugin fires a cost alert when a single event exceeds the threshold.
type Plugin struct {
	enabled      bool
	thresholdUSD float64
	webhookURL   string
	client       *http.Client
}

func (p *Plugin) Name() string { return "costalert" }

// Init configures the plugin.
// Config keys:
//   - "enabled"       bool    — must be true (default false)
//   - "threshold_usd" float64 — alert when event.CostUSD >= this value
//   - "webhook_url"   string  — optional; POST alert payload here
//   - "timeout_ms"    int     — webhook request timeout (default: 3000)
func (p *Plugin) Init(cfg map[string]any) error {
	if v, ok := cfg["enabled"].(bool); ok {
		p.enabled = v
	}
	if !p.enabled {
		return nil
	}

	switch v := cfg["threshold_usd"].(type) {
	case float64:
		p.thresholdUSD = v
	case int:
		p.thresholdUSD = float64(v)
	}
	if p.thresholdUSD <= 0 {
		return fmt.Errorf("costalert: threshold_usd must be > 0")
	}

	p.webhookURL, _ = cfg["webhook_url"].(string)

	timeoutMS := 3000
	if v, ok := cfg["timeout_ms"].(int); ok && v > 0 {
		timeoutMS = v
	}
	p.client = &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}

	slog.Info("costalert middleware ready",
		"threshold_usd", p.thresholdUSD,
		"webhook", p.webhookURL != "",
	)
	return nil
}

// Process checks event cost and fires an alert if the threshold is exceeded.
// The event is never dropped — alerts are informational only.
func (p *Plugin) Process(_ context.Context, e *providers.UsageEvent) error {
	if !p.enabled || e.CostUSD < p.thresholdUSD {
		return nil
	}

	slog.Warn("cost alert",
		"request_id", e.RequestID,
		"model", e.Model,
		"cost_usd", e.CostUSD,
		"threshold_usd", p.thresholdUSD,
		"user", e.Username,
	)

	if p.webhookURL != "" {
		go p.postAlert(e)
	}
	return nil
}

type alertPayload struct {
	Alert        string  `json:"alert"`
	ThresholdUSD float64 `json:"threshold_usd"`
	providers.UsageEvent
}

func (p *Plugin) postAlert(e *providers.UsageEvent) {
	payload := alertPayload{
		Alert:        "cost_threshold_exceeded",
		ThresholdUSD: p.thresholdUSD,
		UsageEvent:   *e,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("costalert: marshal failed", "err", err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, p.webhookURL, bytes.NewReader(body))
	if err != nil {
		slog.Warn("costalert: build request failed", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		slog.Warn("costalert: webhook failed", "url", p.webhookURL, "err", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		slog.Warn("costalert: webhook non-2xx", "status", resp.StatusCode)
	}
}
