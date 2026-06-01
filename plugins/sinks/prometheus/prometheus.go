// Package prometheus is a SinkPlugin that exposes UsageEvent metrics on a
// Prometheus-compatible HTTP scrape endpoint at /metrics.
package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
	"github.com/dvdthecoder/tokenmeter/plugins/sinks"
)

func init() {
	sinks.Register(&Sink{})
}

// Sink serves a /metrics HTTP endpoint for Prometheus scraping.
type Sink struct {
	reg          *prom.Registry
	tokensInput  *prom.CounterVec
	tokensOutput *prom.CounterVec
	tokensCached *prom.CounterVec
	costUSD      *prom.CounterVec
	latencyMS    *prom.HistogramVec
	server       *http.Server
}

func (s *Sink) Name() string { return "prometheus" }

// Init registers all metric instruments and starts the HTTP server.
// Config keys:
//   - "listen" string — address to serve /metrics on (default "127.0.0.1:9090")
func (s *Sink) Init(cfg map[string]any) error {
	listen, _ := cfg["listen"].(string)
	if listen == "" {
		listen = "127.0.0.1:9090"
	}

	s.reg = prom.NewRegistry()
	if err := s.initInstruments(); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{}))

	s.server = &http.Server{Addr: listen, Handler: mux}
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("prometheus sink: server error", "err", err)
		}
	}()

	slog.Info("prometheus sink ready", "addr", listen)
	return nil
}

// initInstruments registers all counters and histograms on the internal registry.
// Separated so tests can call it without starting an HTTP server.
func (s *Sink) initInstruments() error {
	labels := []string{"model", "provider", "user"}

	s.tokensInput = prom.NewCounterVec(prom.CounterOpts{
		Name: "llm_tokens_input_total",
		Help: "Total input tokens processed.",
	}, labels)
	s.tokensOutput = prom.NewCounterVec(prom.CounterOpts{
		Name: "llm_tokens_output_total",
		Help: "Total output tokens generated.",
	}, labels)
	s.tokensCached = prom.NewCounterVec(prom.CounterOpts{
		Name: "llm_tokens_cached_total",
		Help: "Total prompt-cache-hit tokens.",
	}, labels)
	s.costUSD = prom.NewCounterVec(prom.CounterOpts{
		Name: "llm_cost_usd_total",
		Help: "Total estimated cost in USD.",
	}, labels)
	s.latencyMS = prom.NewHistogramVec(prom.HistogramOpts{
		Name:    "llm_latency_ms",
		Help:    "Request latency in milliseconds.",
		Buckets: []float64{100, 250, 500, 1000, 2500, 5000, 10000, 30000},
	}, labels)

	for _, c := range []prom.Collector{
		s.tokensInput, s.tokensOutput, s.tokensCached, s.costUSD, s.latencyMS,
	} {
		if err := s.reg.Register(c); err != nil {
			return fmt.Errorf("prometheus sink: register metric: %w", err)
		}
	}
	return nil
}

// Write records all metric instruments for the given event.
func (s *Sink) Write(_ context.Context, e providers.UsageEvent) error {
	if s.tokensInput == nil {
		return nil
	}
	lbls := prom.Labels{
		"model":    e.Model,
		"provider": e.Provider,
		"user":     e.Username,
	}
	s.tokensInput.With(lbls).Add(float64(e.TokensInput))
	s.tokensOutput.With(lbls).Add(float64(e.TokensOutput))
	if e.TokensCached > 0 {
		s.tokensCached.With(lbls).Add(float64(e.TokensCached))
	}
	s.costUSD.With(lbls).Add(e.CostUSD)
	s.latencyMS.With(lbls).Observe(float64(e.LatencyMS))
	return nil
}

// Close shuts down the HTTP server gracefully.
func (s *Sink) Close() error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(context.Background())
}
