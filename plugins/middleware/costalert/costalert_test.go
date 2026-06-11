package costalert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

func newPlugin(t *testing.T, cfg map[string]any) *Plugin {
	t.Helper()
	p := &Plugin{}
	if err := p.Init(cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return p
}

var cheapEvent = providers.UsageEvent{
	RequestID: "req-cheap",
	Model:     "claude-haiku-4-5",
	CostUSD:   0.001,
	Timestamp: time.Now(),
}

var expensiveEvent = providers.UsageEvent{
	RequestID: "req-expensive",
	Model:     "claude-opus-4-1",
	Username:  "alice",
	CostUSD:   0.50,
	Timestamp: time.Now(),
}

func TestBelowThresholdNoAlert(t *testing.T) {
	p := newPlugin(t, map[string]any{"enabled": true, "threshold_usd": 0.10})
	e := cheapEvent
	if err := p.Process(context.Background(), &e); err != nil {
		t.Errorf("Process returned error: %v", err)
	}
}

func TestAboveThresholdFiresWebhook(t *testing.T) {
	var fired atomic.Bool
	var payload alertPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &payload)
		fired.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := newPlugin(t, map[string]any{
		"enabled":       true,
		"threshold_usd": 0.10,
		"webhook_url":   srv.URL,
		"timeout_ms":    2000,
	})
	e := expensiveEvent
	if err := p.Process(context.Background(), &e); err != nil {
		t.Errorf("Process returned error: %v", err)
	}

	// Wait for async webhook goroutine.
	deadline := time.Now().Add(2 * time.Second)
	for !fired.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if !fired.Load() {
		t.Fatal("webhook was not called")
	}
	if payload.Alert != "cost_threshold_exceeded" {
		t.Errorf("alert field: got %q want cost_threshold_exceeded", payload.Alert)
	}
	if payload.CostUSD != expensiveEvent.CostUSD {
		t.Errorf("CostUSD: got %f want %f", payload.CostUSD, expensiveEvent.CostUSD)
	}
	if payload.ThresholdUSD != 0.10 {
		t.Errorf("ThresholdUSD: got %f want 0.10", payload.ThresholdUSD)
	}
}

func TestAlertDoesNotDropEvent(t *testing.T) {
	// Process must return nil — the event must not be dropped.
	p := newPlugin(t, map[string]any{"enabled": true, "threshold_usd": 0.001})
	e := expensiveEvent
	if err := p.Process(context.Background(), &e); err != nil {
		t.Errorf("event was dropped (error returned): %v", err)
	}
	// Event fields must be unchanged.
	if e.CostUSD != expensiveEvent.CostUSD {
		t.Errorf("event mutated by costalert middleware")
	}
}

func TestDisabledPluginNoOp(t *testing.T) {
	p := &Plugin{} // not initialised
	e := expensiveEvent
	if err := p.Process(context.Background(), &e); err != nil {
		t.Errorf("disabled plugin returned error: %v", err)
	}
}

func TestMissingThresholdError(t *testing.T) {
	p := &Plugin{}
	err := p.Init(map[string]any{"enabled": true}) // no threshold_usd
	if err == nil {
		t.Error("expected error for missing threshold_usd, got nil")
	}
}

func TestExactThresholdFires(t *testing.T) {
	var fired atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fired.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := newPlugin(t, map[string]any{
		"enabled":       true,
		"threshold_usd": 0.50, // exactly equal to expensiveEvent.CostUSD
		"webhook_url":   srv.URL,
	})
	e := expensiveEvent
	_ = p.Process(context.Background(), &e)

	deadline := time.Now().Add(2 * time.Second)
	for !fired.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !fired.Load() {
		t.Error("expected alert at exact threshold, got none")
	}
}
