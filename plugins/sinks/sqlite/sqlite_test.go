package sqlite

import (
	"strings"
	"testing"
	"time"

	storage "github.com/dvdthecoder/tokenmeter/internal/storage/sqlite"
	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

func openMemDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

var testEvent = providers.UsageEvent{
	RequestID:            "req-001",
	Provider:             "anthropic",
	Model:                "claude-sonnet-4-6",
	Username:             "alice",
	ClientName:           "claude-code-cli",
	ClientVersion:        "2.1.142",
	TokensInput:          100,
	TokensOutput:         50,
	TokensCached:         30,
	TokensCachedCreation: 10,
	LatencyMS:            1500,
	CostUSD:              0.009572,
	Timestamp:            time.Date(2026, 5, 15, 6, 0, 0, 0, time.UTC),
	StreamingMode:        true,
}

func TestInsertAndQuery(t *testing.T) {
	db := openMemDB(t)

	if err := db.Insert(testEvent); err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, err := db.Query(storage.QueryOpts{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Model != "claude-sonnet-4-6" {
		t.Errorf("model: got %q", r.Model)
	}
	if r.TokensInput != 100 || r.TokensOutput != 50 {
		t.Errorf("tokens: in=%d out=%d", r.TokensInput, r.TokensOutput)
	}
	if r.CostUSD != 0.009572 {
		t.Errorf("cost: got %f", r.CostUSD)
	}
	if !r.Streaming {
		t.Error("streaming flag not set")
	}
}

func TestDuplicateIgnored(t *testing.T) {
	db := openMemDB(t)
	_ = db.Insert(testEvent)
	_ = db.Insert(testEvent) // same RequestID — should be silently ignored

	rows, _ := db.Query(storage.QueryOpts{})
	if len(rows) != 1 {
		t.Errorf("expected 1 row (duplicate ignored), got %d", len(rows))
	}
}

func TestQuerySinceFilter(t *testing.T) {
	db := openMemDB(t)

	old := testEvent
	old.RequestID = "req-old"
	old.Timestamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = db.Insert(old)
	_ = db.Insert(testEvent)

	rows, _ := db.Query(storage.QueryOpts{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	})
	if len(rows) != 1 {
		t.Errorf("expected 1 recent row, got %d", len(rows))
	}
}

func TestPurge(t *testing.T) {
	db := openMemDB(t)

	old := testEvent
	old.RequestID = "req-old"
	old.Timestamp = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = db.Insert(old)
	_ = db.Insert(testEvent)

	n, err := db.Purge(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 purged row, got %d", n)
	}
	rows, _ := db.Query(storage.QueryOpts{})
	if len(rows) != 1 {
		t.Errorf("expected 1 remaining row, got %d", len(rows))
	}
}

func TestWriteTable(t *testing.T) {
	db := openMemDB(t)
	_ = db.Insert(testEvent)
	rows, _ := db.Query(storage.QueryOpts{})

	var buf strings.Builder
	storage.WriteTable(&buf, rows)
	out := buf.String()
	if !strings.Contains(out, "claude-sonnet-4-6") {
		t.Errorf("table output missing model: %s", out)
	}
	if !strings.Contains(out, "TOTAL") {
		t.Errorf("table output missing TOTAL: %s", out)
	}
}

func TestWriteCSV(t *testing.T) {
	db := openMemDB(t)
	_ = db.Insert(testEvent)
	rows, _ := db.Query(storage.QueryOpts{})

	var buf strings.Builder
	if err := storage.WriteCSV(&buf, rows); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "claude-sonnet-4-6") {
		t.Errorf("csv missing model: %s", out)
	}
	if !strings.Contains(out, "tokens_input") {
		t.Errorf("csv missing header: %s", out)
	}
}

func TestWriteJSON(t *testing.T) {
	db := openMemDB(t)
	_ = db.Insert(testEvent)
	rows, _ := db.Query(storage.QueryOpts{})

	var buf strings.Builder
	if err := storage.WriteJSON(&buf, rows); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"model"`) {
		t.Errorf("json missing model key: %s", out)
	}
}

func TestSinkInit(t *testing.T) {
	s := &Sink{}
	if err := s.Init(map[string]any{"path": ":memory:"}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer s.Close()
	if !s.enabled {
		t.Error("sink should be enabled after Init")
	}
}
