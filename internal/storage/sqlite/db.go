// Package sqlite manages the local events database.
// Schema is content-blind: only token metadata, never prompt/response text.
package sqlite

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dvdthecoder/tokenmeter/plugins/providers"
	_ "modernc.org/sqlite"
)

// Option configures a DB at open time.
type Option func(*DB)

// WithEncryptionKey enables AES-256-GCM field-level encryption. The raw key
// string is SHA-256 hashed to derive a 32-byte AES key. Pass an empty string
// to disable encryption (default).
func WithEncryptionKey(key string) Option {
	return func(d *DB) {
		if key == "" {
			return
		}
		h := sha256.Sum256([]byte(key))
		d.encKey = h[:]
	}
}

const ddl = `
CREATE TABLE IF NOT EXISTS events (
    id                     TEXT PRIMARY KEY,
    ts                     TEXT NOT NULL,
    session_id             TEXT NOT NULL DEFAULT '',
    provider               TEXT NOT NULL DEFAULT '',
    model                  TEXT NOT NULL DEFAULT '',
    service_id             TEXT NOT NULL DEFAULT '',
    username               TEXT NOT NULL DEFAULT '',
    client_name            TEXT NOT NULL DEFAULT '',
    client_version         TEXT NOT NULL DEFAULT '',
    service_tier           TEXT NOT NULL DEFAULT '',
    inference_geo          TEXT NOT NULL DEFAULT '',
    tokens_input           INTEGER NOT NULL DEFAULT 0,
    tokens_output          INTEGER NOT NULL DEFAULT 0,
    tokens_cached          INTEGER NOT NULL DEFAULT 0,
    tokens_cached_creation INTEGER NOT NULL DEFAULT 0,
    latency_ms             INTEGER NOT NULL DEFAULT 0,
    cost_usd               REAL    NOT NULL DEFAULT 0,
    streaming              INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_events_ts         ON events(ts);
CREATE INDEX IF NOT EXISTS idx_events_service_id ON events(service_id);
CREATE INDEX IF NOT EXISTS idx_events_model      ON events(model);

CREATE TABLE IF NOT EXISTS insights (
    id           TEXT PRIMARY KEY,
    generated_at TEXT NOT NULL,
    window_days  INTEGER NOT NULL DEFAULT 7,
    model        TEXT NOT NULL DEFAULT '',
    content      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_insights_generated_at ON insights(generated_at);
`

// DB wraps a SQLite connection for tokenmeter event storage.
type DB struct {
	db     *sql.DB
	encKey []byte // nil = no encryption; 32 bytes = AES-256 key
}

// Open creates or opens the SQLite database at path, runs schema migrations,
// and enables WAL mode. The directory is created if it does not exist.
func Open(path string, opts ...Option) (*DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite: single writer
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("pragma: %w", err)
	}
	if _, err := db.Exec(ddl); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration: %w", err)
	}
	d := &DB{db: db}
	for _, o := range opts {
		o(d)
	}
	return d, nil
}

// runMigrations applies additive schema changes to existing databases.
func runMigrations(db *sql.DB) error {
	additive := []string{
		`ALTER TABLE events ADD COLUMN session_id TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range additive {
		if _, err := db.Exec(stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_session_id ON events(session_id)`)
	return nil
}

// Close releases the database connection.
func (d *DB) Close() error { return d.db.Close() }

// Insert writes a UsageEvent row. Duplicate request IDs are silently ignored.
// String fields that may contain PII are encrypted when a key is configured.
func (d *DB) Insert(e providers.UsageEvent) error {
	streaming := 0
	if e.StreamingMode {
		streaming = 1
	}
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO events
		  (id, ts, session_id, provider, model, service_id, username, client_name, client_version,
		   service_tier, inference_geo, tokens_input, tokens_output, tokens_cached,
		   tokens_cached_creation, latency_ms, cost_usd, streaming)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.RequestID,
		e.Timestamp.UTC().Format(time.RFC3339),
		d.enc(e.SessionID),
		e.Provider, e.Model, d.enc(e.ServiceID), d.enc(e.Username),
		d.enc(e.ClientName), d.enc(e.ClientVersion),
		e.ServiceTier, e.InferenceGeo,
		e.TokensInput, e.TokensOutput, e.TokensCached, e.TokensCachedCreation,
		e.LatencyMS, e.CostUSD, streaming,
	)
	return err
}

// QueryOpts filters results returned by Query.
type QueryOpts struct {
	Since time.Time // zero = no lower bound
	Until time.Time // zero = no upper bound
	Model string    // empty = all models
	User  string    // empty = all users
	Limit int       // 0 = no limit
}

// Row is a query result row — a subset of UsageEvent for display/export.
type Row struct {
	ID                   string    `json:"id"`
	Timestamp            time.Time `json:"timestamp"`
	SessionID            string    `json:"session_id"`
	Provider             string    `json:"provider"`
	Model                string    `json:"model"`
	Username             string    `json:"username"`
	ClientName           string    `json:"client_name"`
	ClientVersion        string    `json:"client_version"`
	ServiceTier          string    `json:"service_tier"`
	TokensInput          int64     `json:"tokens_input"`
	TokensOutput         int64     `json:"tokens_output"`
	TokensCached         int64     `json:"tokens_cached"`
	TokensCachedCreation int64     `json:"tokens_cached_creation"`
	LatencyMS            int64     `json:"latency_ms"`
	TokensPerSec         float64   `json:"tokens_per_sec"`
	CostUSD              float64   `json:"cost_usd"`
	Streaming            bool      `json:"streaming"`
}

// Stats holds aggregate metrics for a query window.
type Stats struct {
	Requests        int     `json:"requests"`
	TokensInput     int64   `json:"tokens_input"`
	TokensOutput    int64   `json:"tokens_output"`
	TokensCached    int64   `json:"tokens_cached"`
	CostUSD         float64 `json:"cost_usd"`
	LatencyMSAvg    float64 `json:"latency_ms_avg"`
	TokensPerSecAvg float64 `json:"tokens_per_sec_avg"`
	Sessions        int     `json:"sessions"`
}

// Query returns events matching opts, ordered by timestamp descending.
func (d *DB) Query(opts QueryOpts) ([]Row, error) {
	where, args := buildWhere(opts)
	limit := ""
	if opts.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	q := `SELECT id, ts, session_id, provider, model, username, client_name, client_version,
	             service_tier, tokens_input, tokens_output, tokens_cached,
	             tokens_cached_creation, latency_ms, cost_usd, streaming
	      FROM events` + where + ` ORDER BY ts DESC` + limit
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var r Row
		var tsStr string
		var streaming int
		if err := rows.Scan(
			&r.ID, &tsStr, &r.SessionID, &r.Provider, &r.Model, &r.Username,
			&r.ClientName, &r.ClientVersion, &r.ServiceTier,
			&r.TokensInput, &r.TokensOutput, &r.TokensCached,
			&r.TokensCachedCreation, &r.LatencyMS, &r.CostUSD, &streaming,
		); err != nil {
			return nil, err
		}
		r.Timestamp, _ = time.Parse(time.RFC3339, tsStr)
		r.Streaming = streaming != 0
		if r.LatencyMS > 0 {
			r.TokensPerSec = float64(r.TokensOutput) / (float64(r.LatencyMS) / 1000.0)
		}
		r.SessionID = d.dec(r.SessionID)
		r.Username = d.dec(r.Username)
		r.ClientName = d.dec(r.ClientName)
		r.ClientVersion = d.dec(r.ClientVersion)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Purge deletes events older than before. Returns number of rows deleted.
func (d *DB) Purge(before time.Time) (int64, error) {
	res, err := d.db.Exec(`DELETE FROM events WHERE ts < ?`, before.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeUser deletes all events for a specific username. Returns number of rows deleted.
func (d *DB) PurgeUser(username string) (int64, error) {
	res, err := d.db.Exec(`DELETE FROM events WHERE username = ?`, username)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// AutoPurge deletes events older than retentionDays. No-op if retentionDays == 0.
func (d *DB) AutoPurge(retentionDays int) (int64, error) {
	if retentionDays == 0 {
		return 0, nil
	}
	return d.Purge(time.Now().AddDate(0, 0, -retentionDays))
}

// QueryStats returns aggregate metrics for events matching opts.
func (d *DB) QueryStats(opts QueryOpts) (Stats, error) {
	where, args := buildWhere(opts)
	row := d.db.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(tokens_input), 0),
		       COALESCE(SUM(tokens_output), 0),
		       COALESCE(SUM(tokens_cached), 0),
		       COALESCE(SUM(cost_usd), 0),
		       COALESCE(AVG(latency_ms), 0),
		       CASE WHEN SUM(latency_ms) > 0
		            THEN SUM(tokens_output) / (SUM(latency_ms) / 1000.0)
		            ELSE 0 END,
		       COUNT(DISTINCT CASE WHEN session_id != '' THEN session_id END)
		FROM events`+where, args...)
	var s Stats
	err := row.Scan(&s.Requests, &s.TokensInput, &s.TokensOutput, &s.TokensCached,
		&s.CostUSD, &s.LatencyMSAvg, &s.TokensPerSecAvg, &s.Sessions)
	return s, err
}

// WriteTable renders rows as a human-readable aligned table to w.
func WriteTable(w io.Writer, rows []Row) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tSESSION\tMODEL\tCLIENT\tUSER\tIN\tOUT\tCACHED\tTOK/S\tCOST")
	var totalIn, totalOut, totalCached int64
	var totalCost float64
	for _, r := range rows {
		client := r.ClientName
		if r.ClientVersion != "" {
			client += "@" + r.ClientVersion
		}
		sess := r.SessionID
		if len(sess) > 12 {
			sess = sess[:12]
		}
		if sess == "" {
			sess = "—"
		}
		tps := "—"
		if r.TokensPerSec > 0 {
			tps = fmt.Sprintf("%.1f", r.TokensPerSec)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t$%.6f\n",
			r.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			sess, r.Model, client, r.Username,
			r.TokensInput, r.TokensOutput, r.TokensCached, tps, r.CostUSD,
		)
		totalIn += r.TokensInput
		totalOut += r.TokensOutput
		totalCached += r.TokensCached
		totalCost += r.CostUSD
	}
	fmt.Fprintln(tw, strings.Repeat("─", 8)+"\t\t"+strings.Repeat("─", 8)+"\t\t\t\t\t\t\t")
	fmt.Fprintf(tw, "TOTAL (%d)\t\t\t\t\t%d\t%d\t%d\t\t$%.6f\n",
		len(rows), totalIn, totalOut, totalCached, totalCost)
	tw.Flush()
}

// WriteJSON renders rows as a JSON array to w.
func WriteJSON(w io.Writer, rows []Row) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// WriteCSV renders rows as CSV to w.
func WriteCSV(w io.Writer, rows []Row) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"id", "timestamp", "session_id", "provider", "model", "username",
		"client_name", "client_version", "service_tier",
		"tokens_input", "tokens_output", "tokens_cached", "tokens_cached_creation",
		"latency_ms", "tokens_per_sec", "cost_usd", "streaming",
	})
	for _, r := range rows {
		streaming := "false"
		if r.Streaming {
			streaming = "true"
		}
		_ = cw.Write([]string{
			r.ID,
			r.Timestamp.UTC().Format(time.RFC3339),
			r.SessionID,
			r.Provider, r.Model, r.Username, r.ClientName, r.ClientVersion, r.ServiceTier,
			strconv.FormatInt(r.TokensInput, 10),
			strconv.FormatInt(r.TokensOutput, 10),
			strconv.FormatInt(r.TokensCached, 10),
			strconv.FormatInt(r.TokensCachedCreation, 10),
			strconv.FormatInt(r.LatencyMS, 10),
			strconv.FormatFloat(r.TokensPerSec, 'f', 2, 64),
			strconv.FormatFloat(r.CostUSD, 'f', 8, 64),
			streaming,
		})
	}
	cw.Flush()
	return cw.Error()
}

func buildWhere(opts QueryOpts) (string, []any) {
	var clauses []string
	var args []any
	if !opts.Since.IsZero() {
		clauses = append(clauses, "ts >= ?")
		args = append(args, opts.Since.UTC().Format(time.RFC3339))
	}
	if !opts.Until.IsZero() {
		clauses = append(clauses, "ts <= ?")
		args = append(args, opts.Until.UTC().Format(time.RFC3339))
	}
	if opts.Model != "" {
		clauses = append(clauses, "model = ?")
		args = append(args, opts.Model)
	}
	if opts.User != "" {
		clauses = append(clauses, "username = ?")
		args = append(args, opts.User)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// Insight is a stored SLM-generated analysis of recent usage data.
type Insight struct {
	ID          string    `json:"id"`
	GeneratedAt time.Time `json:"generated_at"`
	WindowDays  int       `json:"window_days"`
	Model       string    `json:"model"`
	Content     string    `json:"content"`
}

// InsertInsight stores a generated insight. Duplicate IDs are silently ignored.
func (d *DB) InsertInsight(i Insight) error {
	_, err := d.db.Exec(
		`INSERT OR IGNORE INTO insights (id, generated_at, window_days, model, content)
		 VALUES (?, ?, ?, ?, ?)`,
		i.ID,
		i.GeneratedAt.UTC().Format(time.RFC3339),
		i.WindowDays, i.Model, i.Content,
	)
	return err
}

// LatestInsight returns the most recently generated insight, or nil if none exist.
func (d *DB) LatestInsight() (*Insight, error) {
	row := d.db.QueryRow(
		`SELECT id, generated_at, window_days, model, content
		 FROM insights ORDER BY generated_at DESC LIMIT 1`,
	)
	var i Insight
	var tsStr string
	err := row.Scan(&i.ID, &tsStr, &i.WindowDays, &i.Model, &i.Content)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	i.GeneratedAt, _ = time.Parse(time.RFC3339, tsStr)
	return &i, nil
}

// enc encrypts s with AES-256-GCM and returns a base64url-encoded ciphertext.
// Returns s unchanged if no key is configured or s is empty.
func (d *DB) enc(s string) string {
	if len(d.encKey) == 0 || s == "" {
		return s
	}
	block, err := aes.NewCipher(d.encKey)
	if err != nil {
		return s
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return s
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return s
	}
	ct := gcm.Seal(nonce, nonce, []byte(s), nil)
	return base64.RawURLEncoding.EncodeToString(ct)
}

// dec decrypts a base64url-encoded AES-256-GCM ciphertext produced by enc.
// Returns s unchanged if no key is configured, s is empty, or decryption fails
// (allows reading rows written without encryption).
func (d *DB) dec(s string) string {
	if len(d.encKey) == 0 || s == "" {
		return s
	}
	ct, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return s // not encrypted (legacy row)
	}
	block, err := aes.NewCipher(d.encKey)
	if err != nil {
		return s
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return s
	}
	ns := gcm.NonceSize()
	if len(ct) < ns {
		return s
	}
	plain, err := gcm.Open(nil, ct[:ns], ct[ns:], nil)
	if err != nil {
		return s // wrong key or unencrypted row
	}
	return string(plain)
}
