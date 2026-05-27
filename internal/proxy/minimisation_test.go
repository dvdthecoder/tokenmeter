package proxy

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dvdthecoder/tokenmeter/internal/config"
	"github.com/dvdthecoder/tokenmeter/plugins/providers"
	"github.com/dvdthecoder/tokenmeter/plugins/sinks"
)

// captureSink records every UsageEvent written to it.
type captureSink struct {
	mu     sync.Mutex
	events []providers.UsageEvent
}

func (c *captureSink) Name() string { return "capture" }
func (c *captureSink) Init(_ map[string]any) error { return nil }
func (c *captureSink) Write(_ context.Context, e providers.UsageEvent) error {
	c.mu.Lock()
	c.events = append(c.events, e)
	c.mu.Unlock()
	return nil
}
func (c *captureSink) Close() error { return nil }
func (c *captureSink) last() providers.UsageEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.events[len(c.events)-1]
}

var fullEvent = providers.UsageEvent{
	RequestID:     "req-001",
	SessionID:     "sess-abc",
	ServiceID:     "my-service",
	Username:      "alice",
	ClientName:    "claude-code-cli",
	ClientVersion: "2.1.142",
	Provider:      "anthropic",
	Model:         "claude-sonnet-4-6",
	TokensInput:   100,
	TokensOutput:  50,
	LatencyMS:     1500,
	Timestamp:     time.Now(),
}

func TestDataMinimisationStripsAttribution(t *testing.T) {
	cap := &captureSink{}
	sinks.Register(cap)
	t.Cleanup(func() { sinks.Unregister("capture") })

	cfg := &config.Config{}
	cfg.Privacy.DataMinimisation = true
	p := New(cfg)

	p.emit(context.Background(), fullEvent)

	got := cap.last()
	if got.Username != "" {
		t.Errorf("Username not stripped: %q", got.Username)
	}
	if got.ClientName != "" {
		t.Errorf("ClientName not stripped: %q", got.ClientName)
	}
	if got.ClientVersion != "" {
		t.Errorf("ClientVersion not stripped: %q", got.ClientVersion)
	}
	if got.SessionID != "" {
		t.Errorf("SessionID not stripped: %q", got.SessionID)
	}
	if got.ServiceID != "" {
		t.Errorf("ServiceID not stripped: %q", got.ServiceID)
	}
	// Metrics must be preserved.
	if got.TokensInput != fullEvent.TokensInput || got.TokensOutput != fullEvent.TokensOutput {
		t.Errorf("token counts altered: in=%d out=%d", got.TokensInput, got.TokensOutput)
	}
	if got.Model != fullEvent.Model {
		t.Errorf("model altered: %q", got.Model)
	}
}

func TestDataMinimisationOffPreservesAttribution(t *testing.T) {
	cap := &captureSink{}
	sinks.Register(cap)
	t.Cleanup(func() { sinks.Unregister("capture") })

	cfg := &config.Config{}
	cfg.Privacy.DataMinimisation = false
	p := New(cfg)

	p.emit(context.Background(), fullEvent)

	got := cap.last()
	if got.Username != fullEvent.Username {
		t.Errorf("Username changed unexpectedly: %q", got.Username)
	}
	if got.SessionID != fullEvent.SessionID {
		t.Errorf("SessionID changed unexpectedly: %q", got.SessionID)
	}
}
