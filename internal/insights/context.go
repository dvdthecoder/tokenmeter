// Package insights generates privacy-safe usage analysis via a local Ollama SLM.
// The context sent to Ollama contains only aggregated token counts and costs —
// never any prompt or response content.
package insights

import (
	"fmt"
	"sort"
	"strings"

	storage "github.com/dvdthecoder/tokenmeter/internal/storage/sqlite"
)

// BuildContext aggregates rows into a plain-text prompt context for the SLM.
// Only metadata is included — no prompt or response content.
func BuildContext(rows []storage.Row, windowDays int) string {
	if len(rows) == 0 {
		return fmt.Sprintf("No LLM API events recorded in the last %d days.", windowDays)
	}

	type modelStat struct {
		requests  int
		input     int64
		output    int64
		cached    int64
		cost      float64
		latencies []int64
	}
	type userStat struct {
		requests int
		cost     float64
	}

	models := map[string]*modelStat{}
	users := map[string]*userStat{}
	var totalCost float64
	var totalInput, totalCached int64
	var streaming int

	for _, r := range rows {
		ms := models[r.Model]
		if ms == nil {
			ms = &modelStat{}
			models[r.Model] = ms
		}
		ms.requests++
		ms.input += r.TokensInput
		ms.output += r.TokensOutput
		ms.cached += r.TokensCached
		ms.cost += r.CostUSD
		ms.latencies = append(ms.latencies, r.LatencyMS)

		us := users[r.Username]
		if us == nil {
			us = &userStat{}
			users[r.Username] = us
		}
		us.requests++
		us.cost += r.CostUSD

		totalCost += r.CostUSD
		totalInput += r.TokensInput
		totalCached += r.TokensCached
		if r.Streaming {
			streaming++
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "LLM API usage summary — last %d days\n", windowDays)
	fmt.Fprintf(&sb, "Total: %d requests, $%.4f USD\n\n", len(rows), totalCost)

	// Models sorted by request count desc.
	type modelEntry struct {
		name string
		s    *modelStat
	}
	var mlist []modelEntry
	for name, s := range models {
		mlist = append(mlist, modelEntry{name, s})
	}
	sort.Slice(mlist, func(i, j int) bool { return mlist[i].s.requests > mlist[j].s.requests })

	sb.WriteString("By model:\n")
	for _, e := range mlist {
		s := e.s
		fmt.Fprintf(&sb, "  %-40s %4d req  in=%-8s out=%-8s cached=%-8s  $%.4f  p95=%dms\n",
			e.name, s.requests,
			fmtTokens(s.input), fmtTokens(s.output), fmtTokens(s.cached),
			s.cost, p95(s.latencies),
		)
	}

	// Top users (cap at 5, skip blank/unknown-only).
	type userEntry struct {
		name string
		s    *userStat
	}
	var ulist []userEntry
	for name, s := range users {
		if name != "" {
			ulist = append(ulist, userEntry{name, s})
		}
	}
	sort.Slice(ulist, func(i, j int) bool { return ulist[i].s.requests > ulist[j].s.requests })
	if len(ulist) > 5 {
		ulist = ulist[:5]
	}
	if len(ulist) > 0 {
		sb.WriteString("\nBy user (top 5):\n")
		for _, e := range ulist {
			fmt.Fprintf(&sb, "  %-20s %4d req  $%.4f\n", e.name, e.s.requests, e.s.cost)
		}
	}

	cacheRate := 0.0
	if totalInput > 0 {
		cacheRate = float64(totalCached) / float64(totalInput) * 100
	}
	streamRate := 0.0
	if len(rows) > 0 {
		streamRate = float64(streaming) / float64(len(rows)) * 100
	}
	fmt.Fprintf(&sb, "\nCache hit rate: %.1f%%\nStreaming: %.1f%% of requests\n",
		cacheRate, streamRate)

	return sb.String()
}

func p95(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]int64, len(vals))
	copy(sorted, vals)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[int(float64(len(sorted))*0.95+0.5)%len(sorted)]
}

func fmtTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
