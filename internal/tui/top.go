// Package tui provides the live terminal dashboard for tokenmeter top.
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dvdthecoder/tokenmeter/internal/daemon"
	storage "github.com/dvdthecoder/tokenmeter/internal/storage/sqlite"
)

const (
	maxEvents = 200
	pollEvery = 2 * time.Second

	// fixed layout rows: title(1) + blank(1) + stats(1) + blank(1) + tableHeader(1) + sep(1) = 6
	// footer: blank(1) + keys(1) = 2
	fixedRows = 8
)

// ── styles ───────────────────────────────────────────────────────────────────

var (
	bold   = lipgloss.NewStyle().Bold(true)
	faint  = lipgloss.NewStyle().Faint(true)
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	cyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// ── messages ─────────────────────────────────────────────────────────────────

type tickMsg time.Time

// ── model ────────────────────────────────────────────────────────────────────

type liveStats struct {
	requests     int
	tokensIn     int64
	tokensOut    int64
	tokensCached int64
	costUSD      float64
	latencySum   int64
}

// TopModel is the bubbletea model for tokenmeter top.
type TopModel struct {
	db         *storage.DB
	events     []storage.Row // newest first, capped at maxEvents
	stats      liveStats
	since      time.Time
	lastSeen   time.Time // poll watermark
	cursor     int       // first visible row index
	width      int
	height     int
	proxyAlive bool
	proxyPID   int
	errMsg     string
}

// NewTop creates a TopModel. db must already be open.
func NewTop(db *storage.DB) *TopModel {
	now := time.Now()
	m := &TopModel{
		db:       db,
		since:    now,
		lastSeen: now.Add(-pollEvery), // include any events from the last tick on first load
		width:    120,
		height:   24,
	}
	m.proxyPID, m.proxyAlive = daemon.ReadPID()
	return m
}

func (m TopModel) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(pollEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m TopModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampCursor()

	case tickMsg:
		m.proxyPID, m.proxyAlive = daemon.ReadPID()
		m.poll()
		return m, tick()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			m.reset()
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			m.cursor++
			m.clampCursor()
		case "g", "home":
			m.cursor = 0
		case "G", "end":
			m.cursor = max(0, len(m.events)-m.visibleRows())
		}
	}
	return m, nil
}

func (m *TopModel) poll() {
	rows, err := m.db.Query(storage.QueryOpts{
		Since: m.lastSeen,
		Limit: 0,
	})
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	m.errMsg = ""
	m.lastSeen = time.Now()

	if len(rows) == 0 {
		return
	}

	// rows are newest-first from DB; prepend to our buffer (already newest-first)
	m.events = append(rows, m.events...)
	if len(m.events) > maxEvents {
		m.events = m.events[:maxEvents]
	}

	for _, r := range rows {
		m.stats.requests++
		m.stats.tokensIn += r.TokensInput
		m.stats.tokensOut += r.TokensOutput
		m.stats.tokensCached += r.TokensCached
		m.stats.costUSD += r.CostUSD
		m.stats.latencySum += r.LatencyMS
	}
	m.clampCursor()
}

func (m *TopModel) reset() {
	m.events = nil
	m.stats = liveStats{}
	m.since = time.Now()
	m.lastSeen = m.since
	m.cursor = 0
}

func (m *TopModel) visibleRows() int {
	v := m.height - fixedRows
	if v < 1 {
		return 1
	}
	return v
}

func (m *TopModel) clampCursor() {
	max := len(m.events) - m.visibleRows()
	if max < 0 {
		max = 0
	}
	if m.cursor > max {
		m.cursor = max
	}
}

func (m TopModel) View() string {
	var b strings.Builder

	// ── title row ─────────────────────────────────────────────────────────────
	title := bold.Render("tokenmeter top")
	proxyStr := proxyStatus(m.proxyAlive, m.proxyPID)
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(proxyStr) - 2
	if gap < 1 {
		gap = 1
	}
	b.WriteString(title + strings.Repeat(" ", gap) + proxyStr + "\n")
	b.WriteString("\n")

	// ── stats bar ─────────────────────────────────────────────────────────────
	var tps string
	if m.stats.latencySum > 0 {
		tps = fmt.Sprintf("%.1f", float64(m.stats.tokensOut)/(float64(m.stats.latencySum)/1000.0))
	} else {
		tps = "—"
	}
	sinceStr := faint.Render("since " + m.since.Format("15:04:05"))
	statsStr := fmt.Sprintf(
		"  req: %s   in: %s   out: %s   cached: %s   cost: %s   tok/s: %s",
		cyan.Render(fmt.Sprintf("%d", m.stats.requests)),
		formatTokens(m.stats.tokensIn),
		formatTokens(m.stats.tokensOut),
		formatTokens(m.stats.tokensCached),
		yellow.Render(fmt.Sprintf("$%.4f", m.stats.costUSD)),
		formatTPS(tps),
	)
	b.WriteString(sinceStr + statsStr + "\n")
	b.WriteString("\n")

	// ── table header ──────────────────────────────────────────────────────────
	b.WriteString(faint.Render(tableHeader()) + "\n")
	b.WriteString(faint.Render(strings.Repeat("─", min(m.width, 110))) + "\n")

	// ── event rows ────────────────────────────────────────────────────────────
	visible := m.visibleRows()
	end := m.cursor + visible
	if end > len(m.events) {
		end = len(m.events)
	}

	if len(m.events) == 0 {
		b.WriteString(faint.Render("  waiting for events…") + "\n")
	} else {
		for _, r := range m.events[m.cursor:end] {
			b.WriteString(formatRow(r) + "\n")
		}
	}

	// fill remaining lines so footer stays at bottom
	rendered := end - m.cursor
	if rendered < visible {
		b.WriteString(strings.Repeat("\n", visible-rendered))
	}

	// ── footer ────────────────────────────────────────────────────────────────
	b.WriteString("\n")
	footer := faint.Render("q quit  r reset  ↑/k ↓/j scroll  g/G top/bottom")
	if m.errMsg != "" {
		footer = red.Render("error: " + m.errMsg)
	}
	b.WriteString(footer)

	return b.String()
}

// ── helpers ──────────────────────────────────────────────────────────────────

func proxyStatus(alive bool, pid int) string {
	if alive {
		return green.Render(fmt.Sprintf("● proxy running  pid %d", pid))
	}
	return red.Render("○ proxy stopped")
}

func tableHeader() string {
	return fmt.Sprintf("  %-16s %-12s %-26s %-12s %7s %7s %10s %7s",
		"TIME", "PROVIDER", "MODEL", "USER", "IN", "OUT", "COST", "TOK/S")
}

func formatRow(r storage.Row) string {
	t := r.Timestamp.Local().Format("15:04:05")
	model := r.Model
	if len(model) > 24 {
		model = model[:23] + "…"
	}
	user := r.Username
	if user == "" {
		user = faint.Render("—")
	}
	if len(user) > 10 {
		user = user[:9] + "…"
	}
	tps := "—"
	if r.TokensPerSec > 0 {
		tps = fmt.Sprintf("%.1f", r.TokensPerSec)
	}
	return fmt.Sprintf("  %-16s %-12s %-26s %-12s %7s %7s %10s %7s",
		t, r.Provider, model, user,
		formatTokens(r.TokensInput),
		formatTokens(r.TokensOutput),
		yellow.Render(fmt.Sprintf("$%.4f", r.CostUSD)),
		tps,
	)
}

func formatTokens(n int64) string {
	if n == 0 {
		return faint.Render("—")
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatTPS(tps string) string {
	if tps == "—" {
		return faint.Render(tps)
	}
	return cyan.Render(tps)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
