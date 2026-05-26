// Package otel is a SinkPlugin that pushes UsageEvents as OpenTelemetry metrics
// via OTLP gRPC to a central collector.
package otel

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
	"github.com/dvdthecoder/tokenmeter/plugins/sinks"
)

func init() {
	sinks.Register(&Sink{})
}

// Sink pushes UsageEvents as OTLP metrics to a remote collector.
type Sink struct {
	provider     *sdkmetric.MeterProvider
	tokensInput  otelmetric.Int64Counter
	tokensOutput otelmetric.Int64Counter
	tokensCached otelmetric.Int64Counter
	costUSD      otelmetric.Float64Counter
	latencyMS    otelmetric.Int64Histogram
}

func (s *Sink) Name() string { return "otel" }

// Init creates the OTLP gRPC exporter and registers all metric instruments.
// Config keys: "endpoint" (string), "insecure" (bool), "timeout_ms" (int),
// "interval_s" (int — push interval in seconds, default 30).
func (s *Sink) Init(cfg map[string]any) error {
	endpoint, _ := cfg["endpoint"].(string)
	if endpoint == "" {
		endpoint = "localhost:4317"
	}
	tlsInsecure := true
	if v, ok := cfg["insecure"].(bool); ok {
		tlsInsecure = v
	}
	timeoutMS := 5000
	if v, ok := cfg["timeout_ms"].(int); ok && v > 0 {
		timeoutMS = v
	}
	intervalS := 30
	if v, ok := cfg["interval_s"].(int); ok && v > 0 {
		intervalS = v
	}

	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithTimeout(time.Duration(timeoutMS) * time.Millisecond),
	}
	if tlsInsecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exp, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("otel sink: create exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp,
			sdkmetric.WithInterval(time.Duration(intervalS)*time.Second),
		)),
	)
	s.provider = provider

	if err := s.initInstruments(provider.Meter("tokenmeter")); err != nil {
		return err
	}

	slog.Info("otel sink ready", "endpoint", endpoint, "interval_s", intervalS)
	return nil
}

// initInstruments registers all metric instruments on the given meter.
// Separated so tests can inject a ManualReader-backed meter without a real exporter.
func (s *Sink) initInstruments(meter otelmetric.Meter) error {
	var err error

	if s.tokensInput, err = meter.Int64Counter("llm.tokens.input",
		otelmetric.WithDescription("Input tokens processed"),
		otelmetric.WithUnit("{token}"),
	); err != nil {
		return fmt.Errorf("otel: register llm.tokens.input: %w", err)
	}
	if s.tokensOutput, err = meter.Int64Counter("llm.tokens.output",
		otelmetric.WithDescription("Output tokens generated"),
		otelmetric.WithUnit("{token}"),
	); err != nil {
		return fmt.Errorf("otel: register llm.tokens.output: %w", err)
	}
	if s.tokensCached, err = meter.Int64Counter("llm.tokens.cached",
		otelmetric.WithDescription("Cached (prompt cache hit) tokens"),
		otelmetric.WithUnit("{token}"),
	); err != nil {
		return fmt.Errorf("otel: register llm.tokens.cached: %w", err)
	}
	if s.costUSD, err = meter.Float64Counter("llm.cost.usd",
		otelmetric.WithDescription("Estimated cost in USD"),
		otelmetric.WithUnit("{USD}"),
	); err != nil {
		return fmt.Errorf("otel: register llm.cost.usd: %w", err)
	}
	if s.latencyMS, err = meter.Int64Histogram("llm.latency.ms",
		otelmetric.WithDescription("Request latency in milliseconds"),
		otelmetric.WithUnit("{ms}"),
	); err != nil {
		return fmt.Errorf("otel: register llm.latency.ms: %w", err)
	}
	return nil
}

// Write records all five metric instruments for the given event.
func (s *Sink) Write(_ context.Context, e providers.UsageEvent) error {
	if s.tokensInput == nil {
		return nil
	}
	attrs := attribute.NewSet(
		attribute.String("model", e.Model),
		attribute.String("provider", e.Provider),
		attribute.String("user", e.Username),
	)
	opt := otelmetric.WithAttributeSet(attrs)
	ctx := context.Background()

	s.tokensInput.Add(ctx, e.TokensInput, opt)
	s.tokensOutput.Add(ctx, e.TokensOutput, opt)
	if e.TokensCached > 0 {
		s.tokensCached.Add(ctx, e.TokensCached, opt)
	}
	s.costUSD.Add(ctx, e.CostUSD, opt)
	s.latencyMS.Record(ctx, e.LatencyMS, opt)
	return nil
}

// Close flushes pending metrics and shuts down the provider.
// Critical for the ephemeral/sidecar use case — events must reach the collector
// before the process exits.
func (s *Sink) Close() error {
	if s.provider == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.provider.Shutdown(ctx)
}

