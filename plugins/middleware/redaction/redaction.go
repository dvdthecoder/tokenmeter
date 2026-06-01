// Package redaction is a MiddlewarePlugin that scrubs PII from UsageEvents
// using configurable regex patterns before events reach any sink.
package redaction

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/dvdthecoder/tokenmeter/plugins/middleware"
	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

func init() {
	middleware.Register(&Plugin{})
}

// Plugin scrubs configured regex patterns from string fields of UsageEvents.
type Plugin struct {
	enabled  bool
	patterns []*regexp.Regexp
	fields   []string
}

func (p *Plugin) Name() string { return "redaction" }

// Init configures the plugin.
// Config keys:
//   - "enabled"  bool      — must be true for any processing to occur
//   - "patterns" []any     — list of regex strings to match (YAML string slice)
//   - "fields"   []any     — fields to scrub; default ["username","service_id"]
//     Valid field names: username, service_id, session_id, client_name
func (p *Plugin) Init(cfg map[string]any) error {
	if v, ok := cfg["enabled"].(bool); ok {
		p.enabled = v
	}
	if !p.enabled {
		return nil
	}

	// Parse pattern list.
	if raw, ok := cfg["patterns"].([]any); ok {
		for _, r := range raw {
			s, ok := r.(string)
			if !ok {
				continue
			}
			re, err := regexp.Compile(s)
			if err != nil {
				return fmt.Errorf("redaction: invalid pattern %q: %w", s, err)
			}
			p.patterns = append(p.patterns, re)
		}
	}

	// Parse field list; fall back to sensible defaults.
	if raw, ok := cfg["fields"].([]any); ok {
		for _, f := range raw {
			if s, ok := f.(string); ok {
				p.fields = append(p.fields, s)
			}
		}
	}
	if len(p.fields) == 0 {
		p.fields = []string{"username", "service_id"}
	}

	slog.Info("redaction middleware ready",
		"patterns", len(p.patterns),
		"fields", p.fields,
	)
	return nil
}

// Process applies all compiled patterns to the configured fields, replacing
// matches with "[REDACTED]". A pattern error at Process time is a no-op for
// that pattern (patterns are validated at Init time, so this cannot happen in
// normal operation).
func (p *Plugin) Process(_ context.Context, e *providers.UsageEvent) error {
	if !p.enabled || len(p.patterns) == 0 {
		return nil
	}
	for _, field := range p.fields {
		switch field {
		case "username":
			e.Username = p.scrub(e.Username)
		case "service_id":
			e.ServiceID = p.scrub(e.ServiceID)
		case "session_id":
			e.SessionID = p.scrub(e.SessionID)
		case "client_name":
			e.ClientName = p.scrub(e.ClientName)
		}
	}
	return nil
}

func (p *Plugin) scrub(s string) string {
	for _, re := range p.patterns {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}
