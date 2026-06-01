package redaction

import (
	"context"
	"testing"

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

var baseEvent = providers.UsageEvent{
	Username:  "alice@example.com",
	ServiceID: "workspace/production/alice",
	SessionID: "sess-abc123",
	ClientName: "claude-code-cli",
	Model:     "claude-sonnet-4-6",
	Provider:  "anthropic",
}

func TestRedactionScrubsUsername(t *testing.T) {
	p := newPlugin(t, map[string]any{
		"enabled":  true,
		"patterns": []any{`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`},
		"fields":   []any{"username"},
	})
	e := baseEvent
	if err := p.Process(context.Background(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Username != "[REDACTED]" {
		t.Errorf("username not redacted: %q", e.Username)
	}
	// Non-targeted fields must be unchanged.
	if e.ServiceID != baseEvent.ServiceID {
		t.Errorf("service_id changed unexpectedly")
	}
}

func TestRedactionScrubsMultipleFields(t *testing.T) {
	p := newPlugin(t, map[string]any{
		"enabled":  true,
		"patterns": []any{`production`},
		"fields":   []any{"username", "service_id"},
	})
	e := providers.UsageEvent{
		Username:  "production-user",
		ServiceID: "workspace/production/env",
	}
	_ = p.Process(context.Background(), &e)
	if e.Username != "[REDACTED]-user" {
		t.Errorf("username: got %q", e.Username)
	}
	if e.ServiceID != "workspace/[REDACTED]/env" {
		t.Errorf("service_id: got %q", e.ServiceID)
	}
}

func TestRedactionDefaultFields(t *testing.T) {
	// When fields not specified, defaults are username + service_id.
	p := newPlugin(t, map[string]any{
		"enabled":  true,
		"patterns": []any{`alice`},
	})
	e := providers.UsageEvent{
		Username:  "alice",
		ServiceID: "alice-workspace",
		SessionID: "alice-session",
	}
	_ = p.Process(context.Background(), &e)
	if e.Username != "[REDACTED]" {
		t.Errorf("username: got %q", e.Username)
	}
	if e.ServiceID != "[REDACTED]-workspace" {
		t.Errorf("service_id: got %q", e.ServiceID)
	}
	// session_id not in default fields — must be unchanged.
	if e.SessionID != "alice-session" {
		t.Errorf("session_id changed unexpectedly: %q", e.SessionID)
	}
}

func TestRedactionDisabledIsNoop(t *testing.T) {
	p := newPlugin(t, map[string]any{
		"enabled":  false,
		"patterns": []any{`alice`},
	})
	e := baseEvent
	_ = p.Process(context.Background(), &e)
	if e.Username != baseEvent.Username {
		t.Errorf("disabled plugin modified username: %q", e.Username)
	}
}

func TestRedactionNoPatternsIsNoop(t *testing.T) {
	p := newPlugin(t, map[string]any{
		"enabled": true,
		// no patterns key
	})
	e := baseEvent
	_ = p.Process(context.Background(), &e)
	if e.Username != baseEvent.Username {
		t.Errorf("no-pattern plugin modified username: %q", e.Username)
	}
}

func TestRedactionInvalidPatternReturnsError(t *testing.T) {
	p := &Plugin{}
	err := p.Init(map[string]any{
		"enabled":  true,
		"patterns": []any{`[invalid(`},
	})
	if err == nil {
		t.Error("expected error for invalid regex, got nil")
	}
}

func TestRedactionMultiplePatterns(t *testing.T) {
	p := newPlugin(t, map[string]any{
		"enabled":  true,
		"patterns": []any{`alice`, `example\.com`},
		"fields":   []any{"username"},
	})
	e := providers.UsageEvent{Username: "alice@example.com"}
	_ = p.Process(context.Background(), &e)
	// Both patterns apply sequentially.
	if e.Username == "alice@example.com" {
		t.Error("username not redacted at all")
	}
}
