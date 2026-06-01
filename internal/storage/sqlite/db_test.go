package sqlite

import (
	"testing"
	"time"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

func openMemDB(t *testing.T, opts ...Option) *DB {
	t.Helper()
	db, err := Open(":memory:", opts...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

var testEvent = providers.UsageEvent{
	RequestID:    "req-enc-001",
	Provider:     "anthropic",
	Model:        "claude-sonnet-4-6",
	SessionID:    "sess-abc",
	ServiceID:    "svc-xyz",
	Username:     "alice",
	ClientName:   "claude-code",
	ClientVersion: "1.2.3",
	TokensInput:  100,
	TokensOutput: 50,
	CostUSD:      0.009,
	LatencyMS:    1200,
	Timestamp:    time.Now(),
}

func TestEncryptionRoundTrip(t *testing.T) {
	db := openMemDB(t, WithEncryptionKey("test-secret-key"))
	if err := db.Insert(testEvent); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	rows, err := db.Query(QueryOpts{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Username != testEvent.Username {
		t.Errorf("Username: got %q, want %q", r.Username, testEvent.Username)
	}
	if r.ClientName != testEvent.ClientName {
		t.Errorf("ClientName: got %q, want %q", r.ClientName, testEvent.ClientName)
	}
	if r.ClientVersion != testEvent.ClientVersion {
		t.Errorf("ClientVersion: got %q, want %q", r.ClientVersion, testEvent.ClientVersion)
	}
	if r.SessionID != testEvent.SessionID {
		t.Errorf("SessionID: got %q, want %q", r.SessionID, testEvent.SessionID)
	}
}

func TestEncryptionStoredValuesAreObfuscated(t *testing.T) {
	db := openMemDB(t, WithEncryptionKey("test-secret-key"))
	if err := db.Insert(testEvent); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Read raw from sqlite — bypass the dec() layer by using a plain DB.
	rawDB := &DB{db: db.db} // no encKey
	rows, err := rawDB.Query(QueryOpts{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Username == testEvent.Username {
		t.Error("Username stored in plaintext — expected encrypted ciphertext")
	}
	if r.SessionID == testEvent.SessionID {
		t.Error("SessionID stored in plaintext — expected encrypted ciphertext")
	}
}

func TestNoEncryptionStoresPlaintext(t *testing.T) {
	db := openMemDB(t) // no key
	if err := db.Insert(testEvent); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	rows, err := db.Query(QueryOpts{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Username != testEvent.Username {
		t.Errorf("Username: got %q, want %q", rows[0].Username, testEvent.Username)
	}
}

func TestMixedEncryptionGracefulFallback(t *testing.T) {
	// Write without key, read with key — legacy rows should still come back as-is.
	db := openMemDB(t) // plaintext write
	if err := db.Insert(testEvent); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	encDB := &DB{db: db.db}
	WithEncryptionKey("new-key")(encDB)

	rows, err := encDB.Query(QueryOpts{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// dec() falls back to raw value when decryption fails — should return original plaintext
	if rows[0].Username != testEvent.Username {
		t.Errorf("fallback: got %q, want %q", rows[0].Username, testEvent.Username)
	}
}

func TestEncKeyDerivationIsDeterministic(t *testing.T) {
	// Two DBs with same key string should produce identical enc output (nonce aside).
	// We can verify this by writing with one and reading with another opened with same key.
	db1 := openMemDB(t, WithEncryptionKey("same-key"))
	db2 := openMemDB(t, WithEncryptionKey("same-key"))

	if err := db1.Insert(testEvent); err != nil {
		t.Fatalf("db1 Insert: %v", err)
	}

	// Copy raw rows from db1 into db2 to simulate backup/restore.
	raw := &DB{db: db1.db}
	rows, _ := raw.Query(QueryOpts{})
	// Just verify db2 can decrypt what db1 wrote by running enc+dec on a known value.
	val := "hello"
	enc1 := db1.enc(val)
	dec1 := db1.dec(enc1)
	dec2 := db2.dec(enc1)

	if dec1 != val {
		t.Errorf("db1 round-trip: got %q want %q", dec1, val)
	}
	if dec2 != val {
		t.Errorf("db2 cross-decrypt: got %q want %q", dec2, val)
	}
	_ = rows
}

func TestQueryStatsTokensPerSec(t *testing.T) {
	db := openMemDB(t)
	e := testEvent
	e.RequestID = "req-tps-001"
	e.TokensOutput = 100
	e.LatencyMS = 2000 // 2 seconds → 50 tok/s
	if err := db.Insert(e); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	stats, err := db.QueryStats(QueryOpts{})
	if err != nil {
		t.Fatalf("QueryStats: %v", err)
	}
	if stats.TokensPerSecAvg != 50.0 {
		t.Errorf("TokensPerSecAvg: got %.2f, want 50.00", stats.TokensPerSecAvg)
	}
}
