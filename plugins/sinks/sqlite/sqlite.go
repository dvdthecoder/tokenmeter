// Package sqlite is a SinkPlugin that persists UsageEvents to a local SQLite database.
package sqlite

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/yourorg/tokenmeter/internal/config"
	storage "github.com/yourorg/tokenmeter/internal/storage/sqlite"
	"github.com/yourorg/tokenmeter/plugins/providers"
	"github.com/yourorg/tokenmeter/plugins/sinks"
)

func init() {
	sinks.Register(&Sink{})
}

// writeQueueSize is the number of events that can be buffered before Write()
// starts dropping. At typical rates (<10 req/s) this is essentially infinite.
const writeQueueSize = 512

// Sink persists UsageEvents to SQLite via an async write queue so that DB I/O
// never blocks the proxy response path.
type Sink struct {
	db      *storage.DB
	enabled bool
	queue   chan providers.UsageEvent
	done    chan struct{}
}

func (s *Sink) Name() string { return "sqlite" }

// Init opens (or creates) the database, runs auto-purge, and starts the
// background write goroutine.
// Config keys: "path" (string), "retention_days" (int).
func (s *Sink) Init(cfg map[string]any) error {
	path, _ := cfg["path"].(string)
	if path == "" {
		path = DefaultDBPath()
	}

	retentionDays := config.Default().Retention.Days // 90 by default
	if v, ok := cfg["retention_days"].(int); ok && v > 0 {
		retentionDays = v
	}

	db, err := storage.Open(path)
	if err != nil {
		return fmt.Errorf("sqlite sink: %w", err)
	}
	s.db = db
	s.queue = make(chan providers.UsageEvent, writeQueueSize)
	s.done = make(chan struct{})
	s.enabled = true

	n, err := db.AutoPurge(retentionDays)
	if err != nil {
		slog.Warn("sqlite auto-purge failed", "err", err)
	} else if n > 0 {
		slog.Info("sqlite auto-purge", "deleted", n, "retention_days", retentionDays)
	}

	go s.writeLoop()
	slog.Info("sqlite sink ready", "path", path)
	return nil
}

// Write enqueues the event for async insertion. If the queue is full the event
// is dropped with a warning — this protects the proxy from stalling on DB I/O.
func (s *Sink) Write(_ context.Context, e providers.UsageEvent) error {
	if !s.enabled || s.db == nil {
		return nil
	}
	select {
	case s.queue <- e:
	default:
		slog.Warn("sqlite write queue full — event dropped", "request_id", e.RequestID)
	}
	return nil
}

// Close drains the write queue and closes the database connection.
func (s *Sink) Close() error {
	if !s.enabled {
		return nil
	}
	close(s.queue)
	<-s.done
	return s.db.Close()
}

// writeLoop is the single goroutine that performs all DB inserts.
func (s *Sink) writeLoop() {
	defer close(s.done)
	for e := range s.queue {
		if err := s.db.Insert(e); err != nil {
			slog.Warn("sqlite insert failed", "err", err, "request_id", e.RequestID)
		}
	}
}

// DefaultDBPath returns the platform default database path.
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "tokenmeter", "events.db")
}
