package insights

import (
	"strings"
	"testing"
	"time"

	storage "github.com/dvdthecoder/tokenmeter/internal/storage/sqlite"
)

func makeRows() []storage.Row {
	now := time.Now()
	return []storage.Row{
		{Model: "claude-sonnet-4-5", Username: "alice", TokensInput: 1000, TokensOutput: 200, TokensCached: 100, CostUSD: 0.005, LatencyMS: 800, Streaming: true, Timestamp: now},
		{Model: "claude-sonnet-4-5", Username: "alice", TokensInput: 2000, TokensOutput: 400, TokensCached: 500, CostUSD: 0.010, LatencyMS: 1200, Streaming: true, Timestamp: now},
		{Model: "gpt-4o", Username: "bob", TokensInput: 500, TokensOutput: 100, CostUSD: 0.003, LatencyMS: 600, Streaming: false, Timestamp: now},
	}
}

func TestBuildContextNonEmpty(t *testing.T) {
	rows := makeRows()
	ctx := BuildContext(rows, 7)
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	if !strings.Contains(ctx, "claude-sonnet-4-5") {
		t.Error("expected model name in context")
	}
	if !strings.Contains(ctx, "gpt-4o") {
		t.Error("expected second model in context")
	}
	if !strings.Contains(ctx, "alice") {
		t.Error("expected username in context")
	}
}

func TestBuildContextEmpty(t *testing.T) {
	ctx := BuildContext(nil, 7)
	if !strings.Contains(ctx, "No LLM API events") {
		t.Errorf("unexpected empty context: %q", ctx)
	}
}

func TestBuildContextContainsWindowDays(t *testing.T) {
	ctx := BuildContext(makeRows(), 14)
	if !strings.Contains(ctx, "14") {
		t.Error("expected window_days in context")
	}
}

func TestBuildContextNeverContainsPrompt(t *testing.T) {
	// Rows have no prompt/response content — verify the context builder doesn't
	// synthesise or hallucinate any content fields.
	rows := makeRows()
	ctx := BuildContext(rows, 7)
	for _, forbidden := range []string{"prompt", "response", "content", "message"} {
		if strings.Contains(strings.ToLower(ctx), forbidden) {
			t.Errorf("context must not reference %q — privacy violation risk", forbidden)
		}
	}
}

func TestBuildContextCacheRate(t *testing.T) {
	ctx := BuildContext(makeRows(), 7)
	if !strings.Contains(ctx, "Cache hit rate") {
		t.Error("expected cache hit rate in context")
	}
}

func TestFmtTokens(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{500, "500"},
		{1500, "1.5K"},
		{1_500_000, "1.5M"},
	}
	for _, c := range cases {
		got := fmtTokens(c.n)
		if got != c.want {
			t.Errorf("fmtTokens(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
