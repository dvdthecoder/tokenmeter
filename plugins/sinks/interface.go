// Package sinks defines the SinkPlugin interface.
// Sinks receive UsageEvents and persist or forward them.
// Multiple sinks can be active simultaneously (fan-out).
package sinks

import (
	"context"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

// SinkPlugin receives UsageEvents for storage or forwarding.
// Register implementations via Register() in an init() function.
type SinkPlugin interface {
	// Name returns the sink identifier, e.g. "sqlite", "otel", "prometheus".
	Name() string

	// Init is called once at proxy startup with the sink's config block.
	Init(config map[string]any) error

	// Write persists or forwards the event. Must be safe for concurrent calls.
	Write(ctx context.Context, event providers.UsageEvent) error

	// Close flushes and releases resources.
	Close() error
}

var registry = map[string]SinkPlugin{}

// Register adds a sink to the global registry. Call from init().
func Register(s SinkPlugin) {
	registry[s.Name()] = s
}

// Get returns a registered sink by name.
func Get(name string) (SinkPlugin, bool) {
	s, ok := registry[name]
	return s, ok
}

// All returns all registered sinks.
func All() map[string]SinkPlugin {
	return registry
}
