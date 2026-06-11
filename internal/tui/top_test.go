package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	storage "github.com/dvdthecoder/tokenmeter/internal/storage/sqlite"
	"github.com/dvdthecoder/tokenmeter/plugins/providers"
)

func openMemDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestTopViewRendersWithoutEvents(t *testing.T) {
	db := openMemDB(t)
	m := NewTop(db)
	m.width = 120
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "tokenmeter top") {
		t.Error("title not found in view")
	}
	if !strings.Contains(view, "waiting for events") {
		t.Error("empty state message not found")
	}
	if !strings.Contains(view, "q quit") {
		t.Error("footer keys not found")
	}
}

func TestTopViewRendersWithEvents(t *testing.T) {
	db := openMemDB(t)
	_ = db.Insert(providers.UsageEvent{
		RequestID:    "req-001",
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-6",
		Username:     "alice",
		TokensInput:  1000,
		TokensOutput: 500,
		CostUSD:      0.019,
		LatencyMS:    1200,
		Timestamp:    time.Now(),
	})

	m := NewTop(db)
	m.width = 120
	m.height = 30
	m.lastSeen = time.Now().Add(-5 * time.Second) // ensure event is in window
	m.poll()

	view := m.View()
	if !strings.Contains(view, "anthropic") {
		t.Error("provider not found in view")
	}
	if !strings.Contains(view, "alice") {
		t.Error("username not found in view")
	}
	if !strings.Contains(view, "req: 1") {
		t.Error("request count not found in view")
	}
}

func TestTopStatsAccumulate(t *testing.T) {
	db := openMemDB(t)
	for i := 0; i < 3; i++ {
		_ = db.Insert(providers.UsageEvent{
			RequestID:    "req-" + string(rune('A'+i)),
			Provider:     "anthropic",
			Model:        "claude-sonnet-4-6",
			TokensInput:  100,
			TokensOutput: 50,
			CostUSD:      0.005,
			LatencyMS:    1000,
			Timestamp:    time.Now(),
		})
	}

	m := NewTop(db)
	m.width = 120
	m.height = 30
	m.lastSeen = time.Now().Add(-5 * time.Second)
	m.poll()

	if m.stats.requests != 3 {
		t.Errorf("requests: got %d want 3", m.stats.requests)
	}
	if m.stats.tokensIn != 300 {
		t.Errorf("tokensIn: got %d want 300", m.stats.tokensIn)
	}
	want := 0.015
	if m.stats.costUSD < want-0.0001 || m.stats.costUSD > want+0.0001 {
		t.Errorf("costUSD: got %.4f want %.4f", m.stats.costUSD, want)
	}
}

func TestTopResetClearsState(t *testing.T) {
	db := openMemDB(t)
	_ = db.Insert(providers.UsageEvent{
		RequestID: "req-001", Provider: "anthropic", Model: "m",
		TokensInput: 100, Timestamp: time.Now(),
	})

	m := NewTop(db)
	m.lastSeen = time.Now().Add(-5 * time.Second)
	m.poll()

	if m.stats.requests != 1 {
		t.Fatalf("expected 1 event after poll, got %d", m.stats.requests)
	}

	m.reset()
	if m.stats.requests != 0 {
		t.Errorf("requests not reset: got %d", m.stats.requests)
	}
	if len(m.events) != 0 {
		t.Errorf("events not cleared: got %d", len(m.events))
	}
}

func TestTopKeyQuit(t *testing.T) {
	db := openMemDB(t)
	m := NewTop(db)
	m.width = 120
	m.height = 30

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	_ = next
	if cmd == nil {
		t.Fatal("expected quit command, got nil")
	}
	// Execute the command — it should return tea.Quit msg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestTopScrollClampsToEventCount(t *testing.T) {
	db := openMemDB(t)
	m := NewTop(db)
	m.width = 120
	m.height = 30

	// Scroll down with no events — cursor should stay at 0.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 0 {
		t.Errorf("cursor should be 0 with no events, got %d", m.cursor)
	}
}

func TestTopWindowResize(t *testing.T) {
	db := openMemDB(t)
	m := NewTop(db)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	nm := next.(TopModel)
	if nm.width != 200 || nm.height != 50 {
		t.Errorf("resize: got %dx%d want 200x50", nm.width, nm.height)
	}
}
