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
	Timestamp    time.Time `json:"ts"`
	Hook         string    `json:"hook"`
	Action       string    `json:"action"`
	LatencyMs    int64     `json:"latency_ms"`
	Detail       string    `json:"detail,omitempty"`
	BytesIn      int64     `json:"bytes_in,omitempty"`
	BytesOut     int64     `json:"bytes_out,omitempty"`
	Category     string    `json:"cat,omitempty"`
	DurationMs   int64     `json:"dur_ms,omitempty"`
	ExitCode     int       `json:"exit,omitempty"`
	PassedCount  int       `json:"pass_count,omitempty"`
	TotalCount   int       `json:"total_count,omitempty"`
	TurnID       string    `json:"turn_id,omitempty"`
	TaskSource   string    `json:"task_source,omitempty"`
	MemoryLayer  string    `json:"memory_layer,omitempty"`
	MemoryOp     string    `json:"memory_op,omitempty"`
	MemoryResult string    `json:"memory_result,omitempty"`
	ResultCount  int       `json:"result_count,omitempty"`
}

// Record appends an event as a JSONL line. Uses O_APPEND for atomic writes.
func Record(path string, e Event) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	if e.TurnID == "" || e.TaskSource == "" {
		turnID, source := currentTaskIdentity()
		if e.TurnID == "" {
			e.TurnID = turnID
		}
		if e.TaskSource == "" {
			e.TaskSource = source
		}
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

func currentTurnID() string {
	turnID, _ := currentTaskIdentity()
	return turnID
}

func currentTaskIdentity() (string, string) {
	for _, key := range []string{
		"CURSOR_TASK_ID",
		"CLAUDE_CODE_TASK_ID",
		"CURSOR_AGENT_TURN_ID",
		"CURSOR_TURN_ID",
		"CLAUDE_SESSION_ID",
	} {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			if len(value) > 120 {
				value = value[:120]
			}
			switch key {
			case "CURSOR_TASK_ID", "CLAUDE_CODE_TASK_ID":
				return value, "exact"
			default:
				return value, "turn"
			}
		}
	}
	return "", ""
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

// LoadAll reads events from a metrics file and any rotated variants
// (e.g. metrics.jsonl.1, metrics.jsonl.2). This ensures the rolling
// window includes data that survived log rotation.
func LoadAll(path string) ([]Event, error) {
	pattern := path + "*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob rotated metrics: %w", err)
	}
	if len(matches) == 0 {
		return nil, nil
	}
	// Sort so the base file comes first, then .1, .2, etc.
	sort.Strings(matches)

	var all []Event
	for _, m := range matches {
		events, err := Load(m)
		if err != nil {
			continue
		}
		all = append(all, events...)
	}
	return all, nil
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

// MemoryLayerStats holds usage and outcome data for a memory layer.
type MemoryLayerStats struct {
	Layer          string
	Total          int
	Searches       int
	Reads          int
	WriteOps       int
	UpdateOps      int
	Hits           int
	Misses         int
	Empty          int
	Unknown        int
	AvgResultCount float64
}

// CheckStats holds aggregated pass/fail data for doctor/health-check/selftest runs.
type CheckStats struct {
	Name              string
	Runs              int
	Passes            int
	Fails             int
	AvgMs             float64
	AssertionPassRate float64
}

// TaskCoverage tracks how many turn/task groups used each adoption path.
type TaskCoverage struct {
	Total          int
	SkillTasks     int
	MCPTasks       int
	SubagentTasks  int
	IronclawTasks  int
	ExactTasks     int
	TurnTasks      int
	ExplicitTasks  int
	HeuristicTasks int
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

	Skills       []SkillStats
	MCPServers   []MCPServerStats
	MemoryLayers []MemoryLayerStats
	Subagents    []FreqEntry
	Checks       []CheckStats
	Tasks        TaskCoverage
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
	s.MemoryLayers = buildMemoryLayerStats(events, since)
	s.Checks = buildCheckStats(events, since)
	s.Tasks = buildTaskCoverage(events, since)

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

	if s.Tasks.Total >= 5 {
		skillCoverage := percentageInt(s.Tasks.SkillTasks, s.Tasks.Total)
		if skillCoverage < 35 {
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "adoption",
				Message:  fmt.Sprintf("Low skill task coverage (%.1f%%). Read and activate domain skills earlier in each task.", skillCoverage),
			})
		}

		mcpCoverage := percentageInt(s.Tasks.MCPTasks, s.Tasks.Total)
		if mcpCoverage < 25 {
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "adoption",
				Message:  fmt.Sprintf("Low MCP task coverage (%.1f%%). Route more research/doc tasks through always-on MCP servers.", mcpCoverage),
			})
		}

		if s.Tasks.SubagentTasks == 0 {
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "adoption",
				Message:  "No subagent usage recorded in this reporting window. Delegate at least one multi-step task to validate routing.",
			})
		}
	}

	skillUses := 0
	for _, skill := range s.Skills {
		skillUses += skill.Uses
	}
	if s.TotalEvents >= 50 && percentageInt(skillUses, s.TotalEvents) < 10 {
		recs = append(recs, Recommendation{
			Severity: "warn",
			Category: "adoption",
			Message:  fmt.Sprintf("Low skill coverage by events (%.1f%%). Skill routing is still too easy to bypass.", percentageInt(skillUses, s.TotalEvents)),
		})
	}

	if len(s.MCPServers) > 0 {
		serverSet := make(map[string]bool)
		for _, entry := range s.MCPServers {
			if entry.Server != "" {
				serverSet[CanonicalMCPServerName(entry.Server)] = true
			}
		}
		if len(serverSet) < 5 {
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "adoption",
				Message:  fmt.Sprintf("Low MCP diversity (%d server(s)). Spread usage across more always-on servers to avoid single-tool habits.", len(serverSet)),
			})
		}
	}

	mem0Uses := 0
	gitKBUses := 0
	contextModeUses := 0
	for _, layer := range s.MemoryLayers {
		switch layer.Layer {
		case MemoryLayerMem0:
			mem0Uses = layer.Total
			known := layer.Hits + layer.Misses + layer.Empty
			if known > 0 {
				hitRate := float64(layer.Hits) / float64(known) * 100
				if hitRate < 40 {
					recs = append(recs, Recommendation{
						Severity: "warn",
						Category: "memory",
						Message:  fmt.Sprintf("Mem0 reported hit rate is low (%.1f%%). Review write quality, metadata, and search phrasing.", hitRate),
					})
				}
			}
		case MemoryLayerContextMode:
			contextModeUses = layer.Total
		case MemoryLayerGitKB:
			gitKBUses = layer.Total
		case MemoryLayerAllPepper:
			recs = append(recs, Recommendation{
				Severity: "warn",
				Category: "memory",
				Message:  "Legacy allPepper memory usage detected. Hard cutover expects zero allPepper activity.",
			})
		}
	}
	if gitKBUses > 0 && mem0Uses == 0 {
		recs = append(recs, Recommendation{
			Severity: "warn",
			Category: "memory",
			Message:  "Git KB reads are being used without any Mem0 usage in this window. Search Mem0 first before falling back to Git-backed docs.",
		})
	}
	if contextModeUses > 0 && mem0Uses == 0 {
		recs = append(recs, Recommendation{
			Severity: "warn",
			Category: "memory",
			Message:  "Context Mode usage is present without Mem0 usage in this window. Hard cutover expects Mem0 to be the primary hot-memory entry point.",
		})
	}

	if s.Tasks.Total >= 5 && len(s.Skills) == 0 {
		recs = append(recs, Recommendation{
			Severity: "warn",
			Category: "adoption",
			Message:  "No skill activations were recorded. Tracking may be incomplete or routing is bypassing skills entirely.",
		})
	}
	if s.TotalEvents >= 50 && len(s.Subagents) == 0 {
		recs = append(recs, Recommendation{
			Severity: "warn",
			Category: "adoption",
			Message:  "No subagent usage recorded in this reporting window. Delegate at least one specialised task so the path stays exercised.",
		})
	}
	if s.TotalEvents >= 50 && s.Tasks.ExplicitTasks == 0 {
		recs = append(recs, Recommendation{
			Severity: "info",
			Category: "adoption",
			Message:  "Task grouping is still heuristic for this window because older events lack explicit turn IDs. Task coverage KPIs will sharpen as new events are recorded.",
		})
	} else if s.Tasks.ExplicitTasks > 0 && s.Tasks.HeuristicTasks > 0 {
		recs = append(recs, Recommendation{
			Severity: "info",
			Category: "adoption",
			Message:  fmt.Sprintf("Task grouping confidence is mixed for this window: exact=%d, turn-based=%d, heuristic=%d.", s.Tasks.ExactTasks, s.Tasks.TurnTasks, s.Tasks.HeuristicTasks),
		})
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

	if len(s.Checks) > 0 {
		b.WriteString("\n## Self-Check Pass Rates\n\n")
		b.WriteString("| Check | Runs | Pass | Fail | Run Pass Rate | Assertion Pass Rate | Avg (ms) |\n")
		b.WriteString("|-------|------|------|------|---------------|---------------------|----------|\n")
		for _, check := range s.Checks {
			runRate := 0.0
			if check.Runs > 0 {
				runRate = float64(check.Passes) / float64(check.Runs) * 100
			}
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %.1f%% | %.1f%% | %.0f |\n",
				check.Name, check.Runs, check.Passes, check.Fails, runRate, check.AssertionPassRate, check.AvgMs))
		}
	}

	if s.Tasks.Total > 0 {
		b.WriteString("\n## Task Adoption Coverage\n\n")
		b.WriteString(fmt.Sprintf("- Skill task coverage: %d / %d = %.1f%%\n", s.Tasks.SkillTasks, s.Tasks.Total, percentageInt(s.Tasks.SkillTasks, s.Tasks.Total)))
		b.WriteString(fmt.Sprintf("- MCP task coverage: %d / %d = %.1f%%\n", s.Tasks.MCPTasks, s.Tasks.Total, percentageInt(s.Tasks.MCPTasks, s.Tasks.Total)))
		b.WriteString(fmt.Sprintf("- IronClaw MCP task coverage: %d / %d = %.1f%%\n", s.Tasks.IronclawTasks, s.Tasks.Total, percentageInt(s.Tasks.IronclawTasks, s.Tasks.Total)))
		b.WriteString(fmt.Sprintf("- Subagent task coverage: %d / %d = %.1f%%\n", s.Tasks.SubagentTasks, s.Tasks.Total, percentageInt(s.Tasks.SubagentTasks, s.Tasks.Total)))
		b.WriteString(fmt.Sprintf("- Task grouping confidence: exact=%d, turn-based=%d, heuristic=%d\n", s.Tasks.ExactTasks, s.Tasks.TurnTasks, s.Tasks.HeuristicTasks))
		b.WriteString(fmt.Sprintf("- Explicit task groups: %d\n", s.Tasks.ExplicitTasks))
		b.WriteString("- Task grouping uses explicit task IDs when available, falls back to turn IDs, and finally to timestamp clustering for older history.\n")
	}

	if len(s.MemoryLayers) > 0 {
		b.WriteString("\n## Memory Layer KPIs\n\n")
		b.WriteString("| Layer | Uses | Search | Read | Write | Update | Hit | Miss | Empty | Unknown | Hit Rate | Avg Result Count |\n")
		b.WriteString("|-------|------|--------|------|-------|--------|-----|------|-------|---------|----------|------------------|\n")
		for _, layer := range s.MemoryLayers {
			hitRate := "n/a"
			known := layer.Hits + layer.Misses + layer.Empty
			if known > 0 {
				hitRate = fmt.Sprintf("%.1f%%", float64(layer.Hits)/float64(known)*100)
			}
			avgCount := "n/a"
			if layer.AvgResultCount > 0 {
				avgCount = fmt.Sprintf("%.1f", layer.AvgResultCount)
			}
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %d | %d | %d | %d | %s | %s |\n",
				layer.Layer, layer.Total, layer.Searches, layer.Reads, layer.WriteOps, layer.UpdateOps, layer.Hits, layer.Misses, layer.Empty, layer.Unknown, hitRate, avgCount))
		}
		b.WriteString("\n- `unknown` means the layer usage was recorded but the final search outcome was not explicitly tracked.\n")
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
	if len(s.Checks) > 0 {
		parts := make([]string, 0, len(s.Checks))
		for _, check := range s.Checks {
			runRate := 0.0
			if check.Runs > 0 {
				runRate = float64(check.Passes) / float64(check.Runs) * 100
			}
			parts = append(parts, fmt.Sprintf("%s=%.0f%%", check.Name, runRate))
		}
		catPart += " | checks=" + strings.Join(parts, ",")
	}
	if s.Tasks.Total > 0 {
		catPart += fmt.Sprintf(" | tasks=%d skill=%.0f%% mcp=%.0f%% iron=%.0f%% sub=%.0f%%",
			s.Tasks.Total,
			percentageInt(s.Tasks.SkillTasks, s.Tasks.Total),
			percentageInt(s.Tasks.MCPTasks, s.Tasks.Total),
			percentageInt(s.Tasks.IronclawTasks, s.Tasks.Total),
			percentageInt(s.Tasks.SubagentTasks, s.Tasks.Total),
		)
	}
	if len(s.MemoryLayers) > 0 {
		parts := make([]string, 0, len(s.MemoryLayers))
		for _, layer := range s.MemoryLayers {
			parts = append(parts, fmt.Sprintf("%s=%d", layer.Layer, layer.Total))
		}
		catPart += " | memory=" + strings.Join(parts, ",")
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
			if isSkillReadEvent(e) {
				continue
			}
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
			server = CanonicalMCPServerName(key[:idx])
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

func buildMemoryLayerStats(events []Event, since time.Time) []MemoryLayerStats {
	acc := make(map[string]*struct {
		total          int
		searches       int
		reads          int
		writes         int
		updates        int
		hits           int
		misses         int
		empty          int
		unknown        int
		resultCountSum int
		resultCountN   int
	})

	for _, e := range events {
		if e.Timestamp.Before(since) {
			continue
		}

		layer := e.MemoryLayer
		op := e.MemoryOp
		result := e.MemoryResult
		resultCount := e.ResultCount

		if layer == "" && e.Category == "mcp" {
			layer, op = InferMemoryContextFromMCPDetail(e.Detail)
		}
		if layer == "" && e.Hook == "sanitize-read" {
			layer, op, result = InferMemoryContextFromReadPath(e.Detail)
		}
		if layer == "" {
			continue
		}

		entry := acc[layer]
		if entry == nil {
			entry = &struct {
				total          int
				searches       int
				reads          int
				writes         int
				updates        int
				hits           int
				misses         int
				empty          int
				unknown        int
				resultCountSum int
				resultCountN   int
			}{}
			acc[layer] = entry
		}
		entry.total++
		switch op {
		case MemoryOpSearch:
			entry.searches++
		case MemoryOpRead:
			entry.reads++
		case MemoryOpWrite:
			entry.writes++
		case MemoryOpUpdate:
			entry.updates++
		}
		switch result {
		case MemoryResultHit:
			entry.hits++
		case MemoryResultMiss:
			entry.misses++
		case MemoryResultEmpty:
			entry.empty++
		case MemoryResultWrite:
			// write/update confirmation is not part of retrieval hit rate
		default:
			entry.unknown++
		}
		if resultCount > 0 {
			entry.resultCountSum += resultCount
			entry.resultCountN++
		}
	}

	out := make([]MemoryLayerStats, 0, len(acc))
	for layer, entry := range acc {
		avgCount := 0.0
		if entry.resultCountN > 0 {
			avgCount = float64(entry.resultCountSum) / float64(entry.resultCountN)
		}
		out = append(out, MemoryLayerStats{
			Layer:          layer,
			Total:          entry.total,
			Searches:       entry.searches,
			Reads:          entry.reads,
			WriteOps:       entry.writes,
			UpdateOps:      entry.updates,
			Hits:           entry.hits,
			Misses:         entry.misses,
			Empty:          entry.empty,
			Unknown:        entry.unknown,
			AvgResultCount: avgCount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		ri := memoryLayerRank(out[i].Layer)
		rj := memoryLayerRank(out[j].Layer)
		if ri != rj {
			return ri < rj
		}
		return out[i].Total > out[j].Total
	})
	return out
}

func memoryLayerRank(layer string) int {
	switch layer {
	case MemoryLayerMem0:
		return 0
	case MemoryLayerContextMode:
		return 1
	case MemoryLayerGitKB:
		return 2
	case MemoryLayerAllPepper:
		return 3
	default:
		return 9
	}
}

func buildCheckStats(events []Event, since time.Time) []CheckStats {
	acc := make(map[string]*struct {
		runs       int
		passes     int
		fails      int
		totalMs    int64
		passCount  int
		assertions int
	})

	for _, e := range events {
		if e.Timestamp.Before(since) || e.Category != "check" || e.Detail == "" {
			continue
		}
		entry := acc[e.Detail]
		if entry == nil {
			entry = &struct {
				runs       int
				passes     int
				fails      int
				totalMs    int64
				passCount  int
				assertions int
			}{}
			acc[e.Detail] = entry
		}
		entry.runs++
		if e.Action == "pass" {
			entry.passes++
		} else {
			entry.fails++
		}
		entry.totalMs += EffectiveDuration(e)
		entry.passCount += e.PassedCount
		entry.assertions += e.TotalCount
	}

	stats := make([]CheckStats, 0, len(acc))
	for name, entry := range acc {
		avgMs := 0.0
		if entry.runs > 0 {
			avgMs = float64(entry.totalMs) / float64(entry.runs)
		}
		assertionRate := 0.0
		if entry.assertions > 0 {
			assertionRate = float64(entry.passCount) / float64(entry.assertions) * 100
		}
		stats = append(stats, CheckStats{
			Name:              name,
			Runs:              entry.runs,
			Passes:            entry.passes,
			Fails:             entry.fails,
			AvgMs:             avgMs,
			AssertionPassRate: assertionRate,
		})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Name < stats[j].Name })
	return stats
}

func buildTaskCoverage(events []Event, since time.Time) TaskCoverage {
	type taskSignals struct {
		skill    bool
		mcp      bool
		ironclaw bool
		subagent bool
		exact    bool
		turn     bool
		explicit bool
	}

	recent := make([]Event, 0, len(events))
	for _, e := range events {
		if e.Timestamp.Before(since) {
			continue
		}
		recent = append(recent, e)
	}
	if len(recent) == 0 {
		return TaskCoverage{}
	}

	sort.Slice(recent, func(i, j int) bool {
		if recent[i].Timestamp.Equal(recent[j].Timestamp) {
			return recent[i].Hook < recent[j].Hook
		}
		return recent[i].Timestamp.Before(recent[j].Timestamp)
	})

	tasks := make(map[string]*taskSignals)
	bucketCounter := 0
	lastBucketTime := time.Time{}
	currentBucketKey := ""

	for idx, e := range recent {
		key := strings.TrimSpace(e.TurnID)
		if key == "" {
			switch {
			case e.Timestamp.IsZero():
				key = fmt.Sprintf("event:%d", idx)
			case currentBucketKey == "" || e.Timestamp.Sub(lastBucketTime) > 10*time.Minute:
				bucketCounter++
				currentBucketKey = fmt.Sprintf("bucket:%d", bucketCounter)
				lastBucketTime = e.Timestamp
				key = currentBucketKey
			default:
				lastBucketTime = e.Timestamp
				key = currentBucketKey
			}
		} else {
			key = "turn:" + key
		}

		entry := tasks[key]
		if entry == nil {
			entry = &taskSignals{}
			tasks[key] = entry
		}
		if strings.HasPrefix(key, "turn:") {
			entry.explicit = true
			if strings.TrimSpace(e.TaskSource) == "exact" {
				entry.exact = true
			} else {
				entry.turn = true
			}
		}
		switch e.Category {
		case "skill":
			if isSkillReadEvent(e) {
				continue
			}
			entry.skill = true
		case "mcp":
			entry.mcp = true
			if eventMCPServer(e) == "ironclaw" {
				entry.ironclaw = true
			}
		case "subagent":
			entry.subagent = true
		}
	}

	var coverage TaskCoverage
	coverage.Total = len(tasks)
	for _, task := range tasks {
		if task.skill {
			coverage.SkillTasks++
		}
		if task.mcp {
			coverage.MCPTasks++
		}
		if task.ironclaw {
			coverage.IronclawTasks++
		}
		if task.subagent {
			coverage.SubagentTasks++
		}
		if task.exact {
			coverage.ExactTasks++
		}
		if task.turn {
			coverage.TurnTasks++
		}
		if task.explicit {
			coverage.ExplicitTasks++
		} else {
			coverage.HeuristicTasks++
		}
	}
	return coverage
}

func isSkillReadEvent(e Event) bool {
	return e.Category == "skill" && strings.TrimSpace(e.Action) == "read"
}

func eventMCPServer(e Event) string {
	detail := strings.TrimSpace(e.Detail)
	if detail == "" {
		return ""
	}
	if idx := strings.Index(detail, ":"); idx > 0 {
		return CanonicalMCPServerName(detail[:idx])
	}
	return ""
}

func percentageInt(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
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
