package webhook

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

var testEvent = providers.UsageEvent{
	RequestID:    "req-wh-001",
	Provider:     "anthropic",
	Model:        "claude-sonnet-4-6",
	TokensInput:  100,
	TokensOutput: 50,
	CostUSD:      0.009,
	LatencyMS:    1200,
	Timestamp:    time.Now(),
}

func newTestSink(t *testing.T, srv *httptest.Server) *Sink {
	t.Helper()
	s := &Sink{}
	cfg := map[string]any{
		"url":        srv.URL,
		"timeout_ms": 2000,
	}
	if err := s.Init(cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestWebhookDeliversEvent(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestSink(t, srv)
	_ = s.Write(context.Background(), testEvent)
	s.Close()

	if len(received) == 0 {
		t.Fatal("no payload received by webhook server")
	}
	var got providers.UsageEvent
	if err := json.Unmarshal(received, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.RequestID != testEvent.RequestID {
		t.Errorf("RequestID: got %q want %q", got.RequestID, testEvent.RequestID)
	}
	if got.CostUSD != testEvent.CostUSD {
		t.Errorf("CostUSD: got %f want %f", got.CostUSD, testEvent.CostUSD)
	}
}

func TestWebhookSetsContentType(t *testing.T) {
	var ct string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestSink(t, srv)
	_ = s.Write(context.Background(), testEvent)
	s.Close()

	if ct != "application/json" {
		t.Errorf("Content-Type: got %q want application/json", ct)
	}
}

func TestWebhookCustomHeaders(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Sink{}
	if err := s.Init(map[string]any{
		"url":     srv.URL,
		"headers": map[string]any{"Authorization": "Bearer test-token"},
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	_ = s.Write(context.Background(), testEvent)
	s.Close()

	if authHeader != "Bearer test-token" {
		t.Errorf("Authorization: got %q want %q", authHeader, "Bearer test-token")
	}
}

func TestWebhookQueuesMultipleEvents(t *testing.T) {
	var count atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestSink(t, srv)
	for i := 0; i < 5; i++ {
		_ = s.Write(context.Background(), testEvent)
	}
	s.Close()

	if count.Load() != 5 {
		t.Errorf("expected 5 requests, got %d", count.Load())
	}
}

func TestWebhookNoopWhenDisabled(t *testing.T) {
	s := &Sink{} // not initialised
	if err := s.Write(context.Background(), testEvent); err != nil {
		t.Errorf("Write on uninitialised sink returned error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close on uninitialised sink returned error: %v", err)
	}
}

func TestWebhookLogsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestSink(t, srv)
	// Should not return error — non-2xx is logged, not fatal.
	if err := s.Write(context.Background(), testEvent); err != nil {
		t.Errorf("Write returned error: %v", err)
	}
	s.Close()
}
