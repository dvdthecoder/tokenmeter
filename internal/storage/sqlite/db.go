// Package sqlite manages the local events database.
// Schema is content-blind: only token metadata, never prompt/response text.
package sqlite

import (
	"database/sql"
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

const ddl = `
CREATE TABLE IF NOT EXISTS events (
    id                     TEXT PRIMARY KEY,
    ts                     TEXT NOT NULL,
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
	db *sql.DB
}

// Open creates or opens the SQLite database at path, runs schema migrations,
// and enables WAL mode. The directory is created if it does not exist.
func Open(path string) (*DB, error) {
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
	return &DB{db: db}, nil
}

// Close releases the database connection.
func (d *DB) Close() error { return d.db.Close() }

// Insert writes a UsageEvent row. Duplicate request IDs are silently ignored.
func (d *DB) Insert(e providers.UsageEvent) error {
	streaming := 0
	if e.StreamingMode {
		streaming = 1
	}
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO events
		  (id, ts, provider, model, service_id, username, client_name, client_version,
		   service_tier, inference_geo, tokens_input, tokens_output, tokens_cached,
		   tokens_cached_creation, latency_ms, cost_usd, streaming)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.RequestID,
		e.Timestamp.UTC().Format(time.RFC3339),
		e.Provider, e.Model, e.ServiceID, e.Username, e.ClientName, e.ClientVersion,
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
	ID                  string    `json:"id"`
	Timestamp           time.Time `json:"timestamp"`
	Provider            string    `json:"provider"`
	Model               string    `json:"model"`
	Username            string    `json:"username"`
	ClientName          string    `json:"client_name"`
	ClientVersion       string    `json:"client_version"`
	ServiceTier         string    `json:"service_tier"`
	TokensInput         int64     `json:"tokens_input"`
	TokensOutput        int64     `json:"tokens_output"`
	TokensCached        int64     `json:"tokens_cached"`
	TokensCachedCreation int64    `json:"tokens_cached_creation"`
	LatencyMS           int64     `json:"latency_ms"`
	CostUSD             float64   `json:"cost_usd"`
	Streaming           bool      `json:"streaming"`
}

// Query returns events matching opts, ordered by timestamp descending.
func (d *DB) Query(opts QueryOpts) ([]Row, error) {
	where, args := buildWhere(opts)
	limit := ""
	if opts.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	q := `SELECT id, ts, provider, model, username, client_name, client_version,
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
			&r.ID, &tsStr, &r.Provider, &r.Model, &r.Username,
			&r.ClientName, &r.ClientVersion, &r.ServiceTier,
			&r.TokensInput, &r.TokensOutput, &r.TokensCached,
			&r.TokensCachedCreation, &r.LatencyMS, &r.CostUSD, &streaming,
		); err != nil {
			return nil, err
		}
		r.Timestamp, _ = time.Parse(time.RFC3339, tsStr)
		r.Streaming = streaming != 0
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

// WriteTable renders rows as a human-readable aligned table to w.
func WriteTable(w io.Writer, rows []Row) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tMODEL\tCLIENT\tUSER\tIN\tOUT\tCACHED\tCOST")
	var totalIn, totalOut, totalCached int64
	var totalCost float64
	for _, r := range rows {
		client := r.ClientName
		if r.ClientVersion != "" {
			client += "@" + r.ClientVersion
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t$%.6f\n",
			r.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			r.Model, client, r.Username,
			r.TokensInput, r.TokensOutput, r.TokensCached, r.CostUSD,
		)
		totalIn += r.TokensInput
		totalOut += r.TokensOutput
		totalCached += r.TokensCached
		totalCost += r.CostUSD
	}
	fmt.Fprintln(tw, strings.Repeat("─", 8)+"\t"+strings.Repeat("─", 8)+"\t\t\t\t\t\t")
	fmt.Fprintf(tw, "TOTAL (%d)\t\t\t\t%d\t%d\t%d\t$%.6f\n",
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
		"id", "timestamp", "provider", "model", "username",
		"client_name", "client_version", "service_tier",
		"tokens_input", "tokens_output", "tokens_cached", "tokens_cached_creation",
		"latency_ms", "cost_usd", "streaming",
	})
	for _, r := range rows {
		streaming := "false"
		if r.Streaming {
			streaming = "true"
		}
		_ = cw.Write([]string{
			r.ID,
			r.Timestamp.UTC().Format(time.RFC3339),
			r.Provider, r.Model, r.Username, r.ClientName, r.ClientVersion, r.ServiceTier,
			strconv.FormatInt(r.TokensInput, 10),
			strconv.FormatInt(r.TokensOutput, 10),
			strconv.FormatInt(r.TokensCached, 10),
			strconv.FormatInt(r.TokensCachedCreation, 10),
			strconv.FormatInt(r.LatencyMS, 10),
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
