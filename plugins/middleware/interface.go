// Package middleware defines the MiddlewarePlugin interface.
// Middleware runs in a chain between the proxy and the sink layer.
// Use cases: PII redaction, cost alerts, rate limiting, request logging.
package middleware

import (
	"context"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

// MiddlewarePlugin transforms or gates UsageEvents before they reach sinks.
// Register implementations via Register() in an init() function.
type MiddlewarePlugin interface {
	// Name returns the middleware identifier, e.g. "redaction", "costalert".
	Name() string

	// Init is called once at proxy startup with the middleware's config block.
	Init(config map[string]any) error

	// Process transforms the event or returns an error to drop it.
	// Returning a non-nil error causes the event to be discarded (not an error log).
	Process(ctx context.Context, event *providers.UsageEvent) error
}

var registry []MiddlewarePlugin

// Register appends middleware to the ordered chain. Call from init().
// Order of registration is order of execution.
func Register(m MiddlewarePlugin) {
	registry = append(registry, m)
}

// Chain returns the ordered middleware list.
func Chain() []MiddlewarePlugin {
	return registry
}
