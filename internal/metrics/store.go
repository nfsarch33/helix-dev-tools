package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Event represents a single hook or command execution metric.
type Event struct {
	Timestamp  time.Time `json:"ts"`
	Hook       string    `json:"hook"`
	Action     string    `json:"action"`
	LatencyMs  int64     `json:"latency_ms"`
	Detail     string    `json:"detail,omitempty"`
	BytesIn    int64     `json:"bytes_in,omitempty"`
	BytesOut   int64     `json:"bytes_out,omitempty"`
	Category   string    `json:"cat,omitempty"`
	DurationMs int64     `json:"dur_ms,omitempty"`
	ExitCode   int       `json:"exit,omitempty"`
}

// Record appends an event as a JSONL line. Uses O_APPEND for atomic writes.
func Record(path string, e Event) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// Load reads all events from a JSONL file.
func Load(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, scanner.Err()
}

// HookStats holds aggregated statistics for a single hook.
type HookStats struct {
	Hook          string
	Total         int
	DenyCount     int
	WarnCount     int
	AllowCount    int
	OtherCount    int
	AvgLatency    float64
	MaxLatency    int64
	TotalBytesIn  int64
	TotalBytesOut int64
	TopDenied     []FreqEntry
}

// FreqEntry pairs a detail string with a count.
type FreqEntry struct {
	Detail string
	Count  int
}

// CategoryStats holds timing analytics for a single category (mcp, shell, skill, etc.).
type CategoryStats struct {
	Category    string
	Count       int
	AvgDuration float64
	MaxDuration int64
	MinDuration int64
	P95Duration int64
	TotalMs     int64
}

// Trend holds period-over-period comparison data.
type Trend struct {
	PrevEvents      int
	PrevIntervRate  float64
	PrevAvgLatency  float64
	EventsDelta     int
	IntervRateDelta float64
	AvgLatDelta     float64
	HasPrev         bool
}

func (t Trend) arrow(delta float64) string {
	if delta > 0.5 {
		return "^"
	}
	if delta < -0.5 {
		return "v"
	}
	return "="
}

// SkillStats holds activation data for a single Agent Skill.
type SkillStats struct {
	Name  string
	Uses  int
	AvgMs float64
}

// MCPServerStats holds per-server MCP call data.
type MCPServerStats struct {
	Server string
	Tool   string
	Uses   int
	AvgMs  float64
}

// Summary holds the full metrics report.
type Summary struct {
	Since       time.Time
	Until       time.Time
	TotalEvents int
	Hooks       []HookStats
	Categories  []CategoryStats
	TopDenied   []FreqEntry
	Trend       Trend

	Skills     []SkillStats
	MCPServers []MCPServerStats
	Subagents  []FreqEntry
}

// Summarise aggregates events since the given time and computes period-over-period trend.
func Summarise(events []Event, since time.Time) *Summary {
	now := time.Now().UTC()
	s := &Summary{Since: since, Until: now}

	duration := now.Sub(since)
	prevStart := since.Add(-duration)

	hookMap := make(map[string]*hookAccumulator)
	denyFreq := make(map[string]int)

	var prevTotal, prevDeny, prevWarn int
	var prevSumLatency int64
	var prevCount int

	for _, e := range events {
		if !e.Timestamp.Before(since) {
			s.TotalEvents++
			acc, ok := hookMap[e.Hook]
			if !ok {
				acc = &hookAccumulator{hook: e.Hook}
				hookMap[e.Hook] = acc
			}
			acc.add(e)
			if e.Action == "deny" && e.Detail != "" {
				denyFreq[e.Detail]++
			}
		} else if !e.Timestamp.Before(prevStart) {
			prevTotal++
			prevSumLatency += e.LatencyMs
			prevCount++
			switch e.Action {
			case "deny":
				prevDeny++
			case "warn":
				prevWarn++
			}
		}
	}

	for _, acc := range hookMap {
		s.Hooks = append(s.Hooks, acc.stats())
	}
	sort.Slice(s.Hooks, func(i, j int) bool {
		return s.Hooks[i].Total > s.Hooks[j].Total
	})

	s.TopDenied = topN(denyFreq, 10)
	s.Categories = buildCategoryStats(events, since)
	s.Skills, s.MCPServers, s.Subagents = buildAdoptionStats(events, since)

	if prevTotal > 0 {
		prevRate := float64(prevDeny+prevWarn) / float64(prevTotal) * 100
		prevAvg := float64(prevSumLatency) / float64(prevCount)

		curDeny, curWarn, curAll := 0, 0, 0
		var curSumLat int64
		for _, h := range s.Hooks {
			curDeny += h.DenyCount
			curWarn += h.WarnCount
			curAll += h.Total
			curSumLat += int64(h.AvgLatency * float64(h.Total))
		}
		curRate := float64(0)
		curAvg := float64(0)
		if curAll > 0 {
			curRate = float64(curDeny+curWarn) / float64(curAll) * 100
			curAvg = float64(curSumLat) / float64(curAll)
		}

		s.Trend = Trend{
			HasPrev:         true,
			PrevEvents:      prevTotal,
			PrevIntervRate:  prevRate,
			PrevAvgLatency:  prevAvg,
			EventsDelta:     s.TotalEvents - prevTotal,
			IntervRateDelta: curRate - prevRate,
			AvgLatDelta:     curAvg - prevAvg,
		}
	}

	return s
}

// Recommendation is an actionable insight derived from metrics analysis.
type Recommendation struct {
	Severity string // "info", "warn", "critical"
	Category string // "intervention", "latency", "performance", "trend"
	Message  string
}

// Analyse examines the summary data and returns actionable recommendations.
func (s *Summary) Analyse() []Recommendation {
	var recs []Recommendation

	totalDeny, totalWarn, totalAll := 0, 0, 0
	for _, h := range s.Hooks {
		totalDeny += h.DenyCount
		totalWarn += h.WarnCount
		totalAll += h.Total
	}

	if totalAll > 0 {
		rate := float64(totalDeny+totalWarn) / float64(totalAll) * 100
		if rate > 10 {
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "intervention",
				Message:  fmt.Sprintf("High intervention rate (%.1f%%). Review deny patterns for false positives.", rate),
			})
		}
	}

	for _, h := range s.Hooks {
		if h.AvgLatency > 50 {
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "latency",
				Message:  fmt.Sprintf("Hook '%s' avg latency %.0fms exceeds 50ms threshold. Profile for optimisation.", h.Hook, h.AvgLatency),
			})
		}
		if h.MaxLatency > 500 {
			recs = append(recs, Recommendation{
				Severity: "info",
				Category: "latency",
				Message:  fmt.Sprintf("Hook '%s' max latency %dms. Likely cold-start; monitor if recurring.", h.Hook, h.MaxLatency),
			})
		}
	}

	for _, c := range s.Categories {
		if c.P95Duration > 2000 && c.Count >= 5 {
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "performance",
				Message:  fmt.Sprintf("Category '%s' P95=%dms (n=%d). Investigate slow operations.", c.Category, c.P95Duration, c.Count),
			})
		}
	}

	if s.Trend.HasPrev {
		if s.Trend.IntervRateDelta > 5 {
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "trend",
				Message:  fmt.Sprintf("Intervention rate increasing by %.1f%% period-over-period.", s.Trend.IntervRateDelta),
			})
		}
		if s.Trend.AvgLatDelta > 20 {
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "trend",
				Message:  fmt.Sprintf("Average latency increasing by %.1fms period-over-period.", s.Trend.AvgLatDelta),
			})
		}
	}

	return recs
}

// Markdown renders the summary as a Markdown report.
func (s *Summary) Markdown() string {
	var b strings.Builder
	b.WriteString("# System Performance Report\n\n")
	b.WriteString(fmt.Sprintf("Period: %s to %s\n", s.Since.Format("2006-01-02"), s.Until.Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("Total events: %d\n\n", s.TotalEvents))

	if s.TotalEvents == 0 {
		b.WriteString("No metrics data for this period.\n")
		return b.String()
	}

	b.WriteString("## Hook Performance\n\n")
	b.WriteString("| Hook | Total | Deny | Warn | Allow | Avg (ms) | Max (ms) |\n")
	b.WriteString("|------|-------|------|------|-------|----------|----------|\n")

	totalDeny := 0
	totalWarn := 0
	totalAll := 0
	for _, h := range s.Hooks {
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %.1f | %d |\n",
			h.Hook, h.Total, h.DenyCount, h.WarnCount, h.AllowCount, h.AvgLatency, h.MaxLatency))
		totalDeny += h.DenyCount
		totalWarn += h.WarnCount
		totalAll += h.Total
	}

	b.WriteString("\n## Intervention Rate\n\n")
	if totalAll > 0 {
		rate := float64(totalDeny+totalWarn) / float64(totalAll) * 100
		b.WriteString(fmt.Sprintf("- Interventions (deny+warn): %d / %d = %.1f%%\n",
			totalDeny+totalWarn, totalAll, rate))
	}

	if len(s.TopDenied) > 0 {
		b.WriteString("\n## Top Blocked Commands/Paths\n\n")
		for i, d := range s.TopDenied {
			b.WriteString(fmt.Sprintf("%d. `%s` (%dx)\n", i+1, d.Detail, d.Count))
		}
	}

	if len(s.Categories) > 0 {
		b.WriteString("\n## Operation Timing by Category\n\n")
		b.WriteString("| Category | Count | Avg (ms) | P95 (ms) | Max (ms) | Total (s) |\n")
		b.WriteString("|----------|-------|----------|----------|----------|-----------|\n")
		for _, c := range s.Categories {
			b.WriteString(fmt.Sprintf("| %s | %d | %.0f | %d | %d | %.1f |\n",
				c.Category, c.Count, c.AvgDuration, c.P95Duration, c.MaxDuration, float64(c.TotalMs)/1000))
		}
	}

	var totalBytesIn, totalBytesOut int64
	for _, h := range s.Hooks {
		totalBytesIn += h.TotalBytesIn
		totalBytesOut += h.TotalBytesOut
	}
	if totalBytesIn > 0 || totalBytesOut > 0 {
		b.WriteString("\n## Data Throughput\n\n")
		b.WriteString(fmt.Sprintf("- Bytes in: %s\n", humanBytes(totalBytesIn)))
		b.WriteString(fmt.Sprintf("- Bytes out: %s\n", humanBytes(totalBytesOut)))
	}

	if s.Trend.HasPrev {
		b.WriteString("\n## Period-over-Period Trend\n\n")
		b.WriteString(fmt.Sprintf("- Events: %d vs %d (prev) %s\n",
			s.TotalEvents, s.Trend.PrevEvents, s.Trend.arrow(float64(s.Trend.EventsDelta))))
		b.WriteString(fmt.Sprintf("- Intervention rate delta: %+.1f%%\n", s.Trend.IntervRateDelta))
		b.WriteString(fmt.Sprintf("- Avg latency delta: %+.1fms\n", s.Trend.AvgLatDelta))
	}

	b.WriteString("\n## Reflection / Self-Improvement Signals\n\n")
	if totalAll > 0 {
		rate := float64(totalDeny+totalWarn) / float64(totalAll) * 100
		if rate > 10 {
			b.WriteString("- HIGH intervention rate (>10%). Review deny patterns for false positives.\n")
		} else if rate > 5 {
			b.WriteString("- MODERATE intervention rate (5-10%). Monitor for emerging patterns.\n")
		} else {
			b.WriteString("- LOW intervention rate (<5%). System operating normally.\n")
		}
	}
	if totalDeny > 0 && len(s.TopDenied) > 0 {
		b.WriteString("- Recurring blocks detected. Consider adding safe alternatives or user guidance.\n")
	}

	maxAvg := float64(0)
	slowHook := ""
	for _, h := range s.Hooks {
		if h.AvgLatency > maxAvg {
			maxAvg = h.AvgLatency
			slowHook = h.Hook
		}
	}
	if maxAvg > 50 {
		b.WriteString(fmt.Sprintf("- SLOW hook: %s (avg %.0fms). Investigate for optimisation.\n", slowHook, maxAvg))
	}
	if s.Trend.HasPrev && s.Trend.IntervRateDelta > 5 {
		b.WriteString("- Intervention rate INCREASING. Review recent deny pattern changes.\n")
	}
	if s.Trend.HasPrev && s.Trend.AvgLatDelta > 20 {
		b.WriteString("- Average latency INCREASING. Profile slow hooks.\n")
	}

	b.WriteString(fmt.Sprintf("\n*Generated: %s*\n", time.Now().UTC().Format(time.RFC3339)))
	return b.String()
}

// Compact returns a single-line summary for embedding in prompts (token-efficient).
func (s *Summary) Compact(days int) string {
	deny, warn, total := 0, 0, 0
	var sumLat int64
	for _, h := range s.Hooks {
		deny += h.DenyCount
		warn += h.WarnCount
		total += h.Total
		sumLat += int64(h.AvgLatency * float64(h.Total))
	}
	avgLat := int64(0)
	if total > 0 {
		avgLat = sumLat / int64(total)
	}
	trend := "stable"
	if s.Trend.HasPrev {
		if s.Trend.IntervRateDelta > 5 || s.Trend.AvgLatDelta > 20 {
			trend = "degrading"
		} else if s.Trend.IntervRateDelta < -5 || s.Trend.AvgLatDelta < -20 {
			trend = "improving"
		}
	} else {
		trend = "no-baseline"
	}
	catPart := ""
	if len(s.Categories) > 0 {
		parts := make([]string, 0, len(s.Categories))
		for _, c := range s.Categories {
			parts = append(parts, fmt.Sprintf("%s=%d@%.0fms", c.Category, c.Count, c.AvgDuration))
		}
		catPart = " | " + strings.Join(parts, " ")
	}
	return fmt.Sprintf("metrics: %d events %dd | deny=%d warn=%d | avg_lat=%dms | trend=%s%s",
		s.TotalEvents, days, deny, warn, avgLat, trend, catPart)
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMG"[exp])
}

type hookAccumulator struct {
	hook        string
	total       int
	deny        int
	warn        int
	allow       int
	other       int
	sumLatency  int64
	maxLatency  int64
	sumBytesIn  int64
	sumBytesOut int64
	denyFreq    map[string]int
}

func (a *hookAccumulator) add(e Event) {
	a.total++
	a.sumLatency += e.LatencyMs
	if e.LatencyMs > a.maxLatency {
		a.maxLatency = e.LatencyMs
	}
	a.sumBytesIn += e.BytesIn
	a.sumBytesOut += e.BytesOut
	switch e.Action {
	case "deny":
		a.deny++
		if a.denyFreq == nil {
			a.denyFreq = make(map[string]int)
		}
		if e.Detail != "" {
			a.denyFreq[e.Detail]++
		}
	case "warn":
		a.warn++
	case "allow":
		a.allow++
	default:
		a.other++
	}
}

func (a *hookAccumulator) stats() HookStats {
	avg := float64(0)
	if a.total > 0 {
		avg = float64(a.sumLatency) / float64(a.total)
	}
	return HookStats{
		Hook:          a.hook,
		Total:         a.total,
		DenyCount:     a.deny,
		WarnCount:     a.warn,
		AllowCount:    a.allow,
		OtherCount:    a.other,
		AvgLatency:    avg,
		MaxLatency:    a.maxLatency,
		TotalBytesIn:  a.sumBytesIn,
		TotalBytesOut: a.sumBytesOut,
		TopDenied:     topN(a.denyFreq, 5),
	}
}

// EffectiveDuration returns the best timing value for an event: DurationMs for
// tracked operations, LatencyMs for hook events (hook overhead), or 0 if no
// timing data is available.
func EffectiveDuration(e Event) int64 {
	if e.DurationMs > 0 {
		return e.DurationMs
	}
	return e.LatencyMs
}

func buildCategoryStats(events []Event, since time.Time) []CategoryStats {
	catDurations := make(map[string][]int64)
	for _, e := range events {
		if e.Timestamp.Before(since) || e.Category == "" {
			continue
		}
		dur := EffectiveDuration(e)
		catDurations[e.Category] = append(catDurations[e.Category], dur)
	}

	stats := make([]CategoryStats, 0, len(catDurations))
	for cat, durations := range catDurations {
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		var total int64
		for _, d := range durations {
			total += d
		}
		cs := CategoryStats{
			Category:    cat,
			Count:       len(durations),
			TotalMs:     total,
			AvgDuration: float64(total) / float64(len(durations)),
			MinDuration: durations[0],
			MaxDuration: durations[len(durations)-1],
		}
		p95Idx := int(float64(len(durations)) * 0.95)
		if p95Idx >= len(durations) {
			p95Idx = len(durations) - 1
		}
		cs.P95Duration = durations[p95Idx]
		stats = append(stats, cs)
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Count > stats[j].Count })
	return stats
}

// buildAdoptionStats extracts skill activations, MCP server:tool breakdowns,
// and subagent invocations from events.
func buildAdoptionStats(events []Event, since time.Time) ([]SkillStats, []MCPServerStats, []FreqEntry) {
	// Skills: cat=="skill" events (from skill-activate hook or cursor-tools track)
	skillAcc := make(map[string]*struct {
		count   int
		totalMs int64
	})
	// MCP: cat=="mcp" events with "server:tool" in Detail
	mcpAcc := make(map[string]*struct {
		count   int
		totalMs int64
	})
	// Subagents: cat=="subagent"
	subFreq := make(map[string]int)

	for _, e := range events {
		if e.Timestamp.Before(since) {
			continue
		}
		switch e.Category {
		case "skill":
			acc, ok := skillAcc[e.Detail]
			if !ok {
				acc = &struct {
					count   int
					totalMs int64
				}{}
				skillAcc[e.Detail] = acc
			}
			acc.count++
			acc.totalMs += EffectiveDuration(e)
		case "mcp":
			key := EnrichToolDetail(e.Detail)
			acc, ok := mcpAcc[key]
			if !ok {
				acc = &struct {
					count   int
					totalMs int64
				}{}
				mcpAcc[key] = acc
			}
			acc.count++
			acc.totalMs += EffectiveDuration(e)
		case "subagent":
			subFreq[e.Detail]++
		}
	}

	skills := make([]SkillStats, 0, len(skillAcc))
	for name, acc := range skillAcc {
		avg := float64(0)
		if acc.count > 0 {
			avg = float64(acc.totalMs) / float64(acc.count)
		}
		skills = append(skills, SkillStats{Name: name, Uses: acc.count, AvgMs: avg})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Uses > skills[j].Uses })

	mcpServers := make([]MCPServerStats, 0, len(mcpAcc))
	for key, acc := range mcpAcc {
		server, tool := "", key
		if idx := strings.Index(key, ":"); idx > 0 {
			server = key[:idx]
			tool = key[idx+1:]
		}
		avg := float64(0)
		if acc.count > 0 {
			avg = float64(acc.totalMs) / float64(acc.count)
		}
		mcpServers = append(mcpServers, MCPServerStats{Server: server, Tool: tool, Uses: acc.count, AvgMs: avg})
	}
	sort.Slice(mcpServers, func(i, j int) bool { return mcpServers[i].Uses > mcpServers[j].Uses })

	subagents := topN(subFreq, 10)
	return skills, mcpServers, subagents
}

func topN(freq map[string]int, n int) []FreqEntry {
	entries := make([]FreqEntry, 0, len(freq))
	for k, v := range freq {
		entries = append(entries, FreqEntry{Detail: k, Count: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})
	if len(entries) > n {
		entries = entries[:n]
	}
	return entries
}
