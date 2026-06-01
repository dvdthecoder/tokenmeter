package prometheus

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

var testEvent = providers.UsageEvent{
	RequestID:    "req-001",
	Provider:     "anthropic",
	Model:        "claude-sonnet-4-6",
	Username:     "alice",
	TokensInput:  100,
	TokensOutput: 50,
	TokensCached: 30,
	CostUSD:      0.009572,
	LatencyMS:    1500,
	Timestamp:    time.Now(),
}

// newTestSink builds a Sink with instruments registered but no HTTP server.
func newTestSink(t *testing.T) *Sink {
	t.Helper()
	s := &Sink{reg: prom.NewRegistry()}
	if err := s.initInstruments(); err != nil {
		t.Fatalf("initInstruments: %v", err)
	}
	return s
}

func scrape(t *testing.T, s *Sink) string {
	t.Helper()
	handler := promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	return string(body)
}

func TestWriteRecordsTokenCounters(t *testing.T) {
	s := newTestSink(t)
	_ = s.Write(context.Background(), testEvent)
	out := scrape(t, s)

	cases := []struct {
		metric string
		want   string
	}{
		{"llm_tokens_input_total", `llm_tokens_input_total{`},
		{"llm_tokens_output_total", `llm_tokens_output_total{`},
		{"llm_tokens_cached_total", `llm_tokens_cached_total{`},
		{"llm_cost_usd_total", `llm_cost_usd_total{`},
		{"llm_latency_ms", `llm_latency_ms_bucket{`},
	}
	for _, c := range cases {
		if !strings.Contains(out, c.want) {
			t.Errorf("metric %q not found in /metrics output", c.metric)
		}
	}
}

func TestWriteInputTokenValue(t *testing.T) {
	s := newTestSink(t)
	_ = s.Write(context.Background(), testEvent)
	out := scrape(t, s)

	// The counter line should contain 100 for input tokens.
	if !strings.Contains(out, `llm_tokens_input_total{model="claude-sonnet-4-6",provider="anthropic",user="alice"} 100`) {
		t.Errorf("unexpected llm_tokens_input_total value in:\n%s", out)
	}
}

func TestWriteAccumulatesAcrossEvents(t *testing.T) {
	s := newTestSink(t)
	_ = s.Write(context.Background(), testEvent)
	_ = s.Write(context.Background(), testEvent)
	out := scrape(t, s)

	if !strings.Contains(out, `llm_tokens_input_total{model="claude-sonnet-4-6",provider="anthropic",user="alice"} 200`) {
		t.Errorf("expected 200 cumulative input tokens:\n%s", out)
	}
}

func TestWriteSkipsCachedWhenZero(t *testing.T) {
	s := newTestSink(t)
	e := testEvent
	e.TokensCached = 0
	_ = s.Write(context.Background(), e)
	out := scrape(t, s)

	// Counter exists but should be 0 (never incremented).
	if strings.Contains(out, `llm_tokens_cached_total{`) {
		// Value line should not appear when 0 — Prometheus omits zero counters.
		if strings.Contains(out, `llm_tokens_cached_total{model=`) {
			t.Errorf("cached counter should not appear for zero value:\n%s", out)
		}
	}
}

func TestWriteNoopWhenNotInitialised(t *testing.T) {
	s := &Sink{}
	if err := s.Write(context.Background(), testEvent); err != nil {
		t.Errorf("Write on uninitialised sink returned error: %v", err)
	}
}

func TestCloseWithNoServer(t *testing.T) {
	s := &Sink{}
	if err := s.Close(); err != nil {
		t.Errorf("Close on sink with no server returned error: %v", err)
	}
}
