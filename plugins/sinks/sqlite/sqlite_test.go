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

func TestPurgeUser(t *testing.T) {
	db := openMemDB(t)

	alice := testEvent
	alice.RequestID = "req-alice"
	alice.Username = "alice"

	bob := testEvent
	bob.RequestID = "req-bob"
	bob.Username = "bob"

	_ = db.Insert(alice)
	_ = db.Insert(bob)

	n, err := db.PurgeUser("alice")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 purged row, got %d", n)
	}

	rows, _ := db.Query(storage.QueryOpts{})
	if len(rows) != 1 {
		t.Errorf("expected 1 remaining row (bob), got %d", len(rows))
	}
	if rows[0].Username != "bob" {
		t.Errorf("expected bob to remain, got %q", rows[0].Username)
	}
}

func TestPurgeUserNotFound(t *testing.T) {
	db := openMemDB(t)
	_ = db.Insert(testEvent)

	n, err := db.PurgeUser("nobody")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows deleted for unknown user, got %d", n)
	}
}

func TestTokensPerSec(t *testing.T) {
	db := openMemDB(t)
	_ = db.Insert(testEvent) // LatencyMS=1500, TokensOutput=50 → 33.33 tok/s

	rows, err := db.Query(storage.QueryOpts{})
	if err != nil {
		t.Fatal(err)
	}
	r := rows[0]
	want := float64(50) / 1.5 // ~33.33
	if r.TokensPerSec < 33.0 || r.TokensPerSec > 34.0 {
		t.Errorf("TokensPerSec: got %.2f, want ~%.2f", r.TokensPerSec, want)
	}
}

func TestTokensPerSecZeroLatency(t *testing.T) {
	db := openMemDB(t)
	e := testEvent
	e.RequestID = "req-zero-lat"
	e.LatencyMS = 0
	_ = db.Insert(e)

	rows, _ := db.Query(storage.QueryOpts{})
	if rows[0].TokensPerSec != 0 {
		t.Errorf("expected 0 tok/s for zero latency, got %.2f", rows[0].TokensPerSec)
	}
}

func TestQueryStatsTokensPerSec(t *testing.T) {
	db := openMemDB(t)
	_ = db.Insert(testEvent) // 50 out / 1500ms

	stats, err := db.QueryStats(storage.QueryOpts{})
	if err != nil {
		t.Fatal(err)
	}
	// SUM(tokens_output)=50, SUM(latency_ms)=1500 → 50/1.5 ≈ 33.33
	if stats.TokensPerSecAvg < 33.0 || stats.TokensPerSecAvg > 34.0 {
		t.Errorf("TokensPerSecAvg: got %.2f, want ~33.33", stats.TokensPerSecAvg)
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
	if !strings.Contains(out, "TOK/S") {
		t.Errorf("table output missing TOK/S column: %s", out)
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
	if !strings.Contains(out, "tokens_per_sec") {
		t.Errorf("csv missing tokens_per_sec header: %s", out)
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

func TestInsertAndLatestInsight(t *testing.T) {
	db := openMemDB(t)
	ins := storage.Insight{
		ID:          "ins_001",
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		WindowDays:  7,
		Model:       "llama3.2:3b",
		Content:     "Usage looks healthy. Consider caching more.",
	}
	if err := db.InsertInsight(ins); err != nil {
		t.Fatalf("InsertInsight: %v", err)
	}
	got, err := db.LatestInsight()
	if err != nil {
		t.Fatalf("LatestInsight: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil insight")
	}
	if got.ID != ins.ID {
		t.Errorf("id: got %q, want %q", got.ID, ins.ID)
	}
	if got.Content != ins.Content {
		t.Errorf("content: got %q, want %q", got.Content, ins.Content)
	}
	if got.Model != ins.Model {
		t.Errorf("model: got %q, want %q", got.Model, ins.Model)
	}
}

func TestLatestInsightEmpty(t *testing.T) {
	db := openMemDB(t)
	got, err := db.LatestInsight()
	if err != nil {
		t.Fatalf("LatestInsight: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil insight on empty DB, got %+v", got)
	}
}

func TestLatestInsightReturnsNewest(t *testing.T) {
	db := openMemDB(t)
	older := storage.Insight{
		ID: "ins_old", GeneratedAt: time.Now().Add(-24 * time.Hour).UTC(),
		WindowDays: 7, Model: "llama3.2:3b", Content: "older",
	}
	newer := storage.Insight{
		ID: "ins_new", GeneratedAt: time.Now().UTC(),
		WindowDays: 7, Model: "llama3.2:3b", Content: "newer",
	}
	_ = db.InsertInsight(older)
	_ = db.InsertInsight(newer)

	got, err := db.LatestInsight()
	if err != nil {
		t.Fatalf("LatestInsight: %v", err)
	}
	if got.ID != "ins_new" {
		t.Errorf("expected newest insight, got %q", got.ID)
	}
}
