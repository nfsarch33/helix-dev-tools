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
	Timestamp time.Time `json:"ts"`
	Hook      string    `json:"hook"`
	Action    string    `json:"action"`
	LatencyMs int64     `json:"latency_ms"`
	Detail    string    `json:"detail,omitempty"`
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
	Hook       string
	Total      int
	DenyCount  int
	WarnCount  int
	AllowCount int
	OtherCount int
	AvgLatency float64
	MaxLatency int64
	TopDenied  []FreqEntry
}

// FreqEntry pairs a detail string with a count.
type FreqEntry struct {
	Detail string
	Count  int
}

// Summary holds the full metrics report.
type Summary struct {
	Since       time.Time
	Until       time.Time
	TotalEvents int
	Hooks       []HookStats
	TopDenied   []FreqEntry
}

// Summarise aggregates events since the given time.
func Summarise(events []Event, since time.Time) *Summary {
	s := &Summary{Since: since, Until: time.Now().UTC()}

	hookMap := make(map[string]*hookAccumulator)
	denyFreq := make(map[string]int)

	for _, e := range events {
		if e.Timestamp.Before(since) {
			continue
		}
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
	}

	for _, acc := range hookMap {
		s.Hooks = append(s.Hooks, acc.stats())
	}
	sort.Slice(s.Hooks, func(i, j int) bool {
		return s.Hooks[i].Total > s.Hooks[j].Total
	})

	s.TopDenied = topN(denyFreq, 10)
	return s
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

	b.WriteString(fmt.Sprintf("\n*Generated: %s*\n", time.Now().UTC().Format(time.RFC3339)))
	return b.String()
}

type hookAccumulator struct {
	hook       string
	total      int
	deny       int
	warn       int
	allow      int
	other      int
	sumLatency int64
	maxLatency int64
	denyFreq   map[string]int
}

func (a *hookAccumulator) add(e Event) {
	a.total++
	a.sumLatency += e.LatencyMs
	if e.LatencyMs > a.maxLatency {
		a.maxLatency = e.LatencyMs
	}
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
		Hook:       a.hook,
		Total:      a.total,
		DenyCount:  a.deny,
		WarnCount:  a.warn,
		AllowCount: a.allow,
		OtherCount: a.other,
		AvgLatency: avg,
		MaxLatency: a.maxLatency,
		TopDenied:  topN(a.denyFreq, 5),
	}
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
