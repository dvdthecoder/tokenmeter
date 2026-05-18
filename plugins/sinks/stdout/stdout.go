package stdout

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
	"github.com/dvdthecoder/tokenmeter/plugins/sinks"
)

func init() {
	sinks.Register(&Sink{})
}

type Sink struct{ enabled bool }

func (s *Sink) Name() string { return "stdout" }

func (s *Sink) Init(config map[string]any) error {
	if v, ok := config["enabled"].(bool); ok {
		s.enabled = v
	}
	return nil
}

func (s *Sink) Write(_ context.Context, e providers.UsageEvent) error {
	if !s.enabled {
		return nil
	}
	stream := ""
	if e.StreamingMode {
		stream = " [stream]"
	}
	tier := ""
	if e.ServiceTier != "" && e.ServiceTier != "standard" {
		tier = " tier=" + e.ServiceTier
	}
	fmt.Fprintf(os.Stderr,
		"[tokenmeter]%s %s  client=%-20s user=%-12s model=%-28s  in=%-6d out=%-6d cached=%-6d cache_write=%-6d  cost=$%.6f  latency=%dms%s\n",
		stream,
		e.Timestamp.UTC().Format(time.RFC3339),
		clientLabel(e.ClientName, e.ClientVersion),
		e.Username,
		e.Model,
		e.TokensInput,
		e.TokensOutput,
		e.TokensCached,
		e.TokensCachedCreation,
		e.CostUSD,
		e.LatencyMS,
		tier,
	)
	return nil
}

func (s *Sink) Close() error { return nil }

func clientLabel(name, version string) string {
	if name == "" {
		return "unknown"
	}
	if version == "" {
		return name
	}
	return name + "@" + version
}
