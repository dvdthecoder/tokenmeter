package insights

import (
	"context"
	"fmt"
	"time"

	"github.com/dvdthecoder/tokenmeter/internal/config"
	storage "github.com/dvdthecoder/tokenmeter/internal/storage/sqlite"
)

// Run generates a new insight from recent events, stores it, and streams tokens
// to onToken as they arrive. Returns the stored Insight on success.
// If Ollama is unreachable the error is returned unwrapped so callers can print
// a friendly skip message rather than treating it as fatal.
func Run(ctx context.Context, db *storage.DB, cfg config.InsightsConfig, onToken func(string)) (*storage.Insight, error) {
	since := time.Now().AddDate(0, 0, -cfg.WindowDays)
	rows, err := db.Query(storage.QueryOpts{Since: since})
	if err != nil {
		return nil, fmt.Errorf("insights: query events: %w", err)
	}

	usageCtx := BuildContext(rows, cfg.WindowDays)
	text, err := Generate(ctx, cfg.OllamaURL, cfg.Model, usageCtx, onToken)
	if err != nil {
		return nil, err // caller decides whether to log+skip or fatal
	}
	if text == "" {
		return nil, fmt.Errorf("insights: Ollama returned empty response")
	}

	insight := storage.Insight{
		ID:          fmt.Sprintf("ins_%d", time.Now().UnixNano()),
		GeneratedAt: time.Now().UTC(),
		WindowDays:  cfg.WindowDays,
		Model:       cfg.Model,
		Content:     text,
	}
	if err := db.InsertInsight(insight); err != nil {
		return nil, fmt.Errorf("insights: store: %w", err)
	}
	return &insight, nil
}
