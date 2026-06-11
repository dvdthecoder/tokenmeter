// Package webhook is a SinkPlugin that POSTs UsageEvents as JSON to a
// configurable HTTP endpoint. Writes are async (buffered channel) so the
// proxy response path is never blocked by HTTP latency.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
	"github.com/dvdthecoder/tokenmeter/plugins/sinks"
)

func init() {
	sinks.Register(&Sink{})
}

const writeQueueSize = 256

// Sink POSTs UsageEvents as JSON to a remote endpoint.
type Sink struct {
	url     string
	method  string
	headers map[string]string
	client  *http.Client
	queue   chan providers.UsageEvent
	done    chan struct{}
	enabled bool
	once    sync.Once
}

func (s *Sink) Name() string { return "webhook" }

// Init configures the sink.
// Config keys:
//   - "url"        string          — destination endpoint (required)
//   - "method"     string          — HTTP method (default: POST)
//   - "timeout_ms" int             — per-request timeout (default: 5000)
//   - "headers"    map[string]any  — extra request headers (e.g. Authorization)
func (s *Sink) Init(cfg map[string]any) error {
	s.url, _ = cfg["url"].(string)
	if s.url == "" {
		slog.Warn("webhook sink: no url configured — sink disabled")
		return nil
	}

	s.method = "POST"
	if v, _ := cfg["method"].(string); v != "" {
		s.method = v
	}

	timeoutMS := 5000
	if v, ok := cfg["timeout_ms"].(int); ok && v > 0 {
		timeoutMS = v
	}
	s.client = &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}

	if raw, ok := cfg["headers"].(map[string]any); ok {
		s.headers = make(map[string]string, len(raw))
		for k, v := range raw {
			if sv, ok := v.(string); ok {
				s.headers[k] = sv
			}
		}
	}

	s.queue = make(chan providers.UsageEvent, writeQueueSize)
	s.done = make(chan struct{})
	s.enabled = true
	go s.writeLoop()

	slog.Info("webhook sink ready", "url", s.url, "method", s.method)
	return nil
}

// Write enqueues the event for async delivery.
func (s *Sink) Write(_ context.Context, e providers.UsageEvent) error {
	if !s.enabled {
		return nil
	}
	select {
	case s.queue <- e:
	default:
		slog.Warn("webhook write queue full — event dropped", "request_id", e.RequestID)
	}
	return nil
}

// Close drains the queue and waits for in-flight requests to finish.
// Safe to call multiple times.
func (s *Sink) Close() error {
	if !s.enabled {
		return nil
	}
	s.once.Do(func() {
		close(s.queue)
		<-s.done
	})
	return nil
}

func (s *Sink) writeLoop() {
	defer close(s.done)
	for e := range s.queue {
		s.post(e)
	}
}

func (s *Sink) post(e providers.UsageEvent) {
	body, err := json.Marshal(e)
	if err != nil {
		slog.Warn("webhook: marshal failed", "err", err)
		return
	}
	req, err := http.NewRequest(s.method, s.url, bytes.NewReader(body))
	if err != nil {
		slog.Warn("webhook: build request failed", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		slog.Warn("webhook: request failed", "url", s.url, "err", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		slog.Warn("webhook: non-2xx response", "url", s.url, "status", resp.StatusCode)
	}
}
