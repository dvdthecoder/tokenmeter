package otel

import (
	"context"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

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

// newTestSink builds a Sink backed by a ManualReader so tests can collect
// metrics without a real OTLP endpoint.
func newTestSink(t *testing.T) (*Sink, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	s := &Sink{provider: provider}
	if err := s.initInstruments(provider.Meter("tokenmeter")); err != nil {
		t.Fatalf("initInstruments: %v", err)
	}
	return s, reader
}

func collect(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	return rm
}

func int64CounterValue(rm metricdata.ResourceMetrics, name string) (int64, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				return 0, false
			}
			var total int64
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
			return total, true
		}
	}
	return 0, false
}

func float64CounterValue(rm metricdata.ResourceMetrics, name string) (float64, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[float64])
			if !ok {
				return 0, false
			}
			var total float64
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
			return total, true
		}
	}
	return 0, false
}

func histogramExists(rm metricdata.ResourceMetrics, name string) bool {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				_, ok := m.Data.(metricdata.Histogram[int64])
				return ok
			}
		}
	}
	return false
}

func TestWriteRecordsTokenCounters(t *testing.T) {
	s, reader := newTestSink(t)
	_ = s.Write(context.Background(), testEvent)
	rm := collect(t, reader)

	cases := []struct{ name string; want int64 }{
		{"llm.tokens.input", 100},
		{"llm.tokens.output", 50},
		{"llm.tokens.cached", 30},
	}
	for _, c := range cases {
		got, ok := int64CounterValue(rm, c.name)
		if !ok {
			t.Errorf("metric %q not found", c.name)
			continue
		}
		if got != c.want {
			t.Errorf("metric %q: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestWriteRecordsCostCounter(t *testing.T) {
	s, reader := newTestSink(t)
	_ = s.Write(context.Background(), testEvent)
	rm := collect(t, reader)

	got, ok := float64CounterValue(rm, "llm.cost.usd")
	if !ok {
		t.Fatal("llm.cost.usd not found")
	}
	if got < 0.009 || got > 0.010 {
		t.Errorf("llm.cost.usd: got %f, want ~0.009572", got)
	}
}

func TestWriteRecordsLatencyHistogram(t *testing.T) {
	s, reader := newTestSink(t)
	_ = s.Write(context.Background(), testEvent)
	rm := collect(t, reader)

	if !histogramExists(rm, "llm.latency.ms") {
		t.Error("llm.latency.ms histogram not found")
	}
}

func TestWriteAccumulatesAcrossEvents(t *testing.T) {
	s, reader := newTestSink(t)
	_ = s.Write(context.Background(), testEvent)
	_ = s.Write(context.Background(), testEvent)
	rm := collect(t, reader)

	got, _ := int64CounterValue(rm, "llm.tokens.input")
	if got != 200 {
		t.Errorf("expected 200 cumulative input tokens after 2 writes, got %d", got)
	}
}

func TestWriteSkipsCachedWhenZero(t *testing.T) {
	s, reader := newTestSink(t)
	e := testEvent
	e.TokensCached = 0
	_ = s.Write(context.Background(), e)
	rm := collect(t, reader)

	got, _ := int64CounterValue(rm, "llm.tokens.cached")
	if got != 0 {
		t.Errorf("expected 0 cached tokens when event has none, got %d", got)
	}
}

func TestWriteNoopWhenNotInitialised(t *testing.T) {
	s := &Sink{} // no initInstruments called
	if err := s.Write(context.Background(), testEvent); err != nil {
		t.Errorf("Write on uninitialised sink returned error: %v", err)
	}
}
