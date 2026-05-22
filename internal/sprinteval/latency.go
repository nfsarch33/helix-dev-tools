package sprinteval

import (
	"math"
	"sort"
)

// LatencyStats holds aggregate latency percentiles for an agentrace event
// set. Durations are in milliseconds. Used for P50/P95 reporting on the
// composite event stream and per-tool histograms.
type LatencyStats struct {
	Count int   `json:"count"`
	MinMs int64 `json:"min_ms"`
	MaxMs int64 `json:"max_ms"`
	P50Ms int64 `json:"p50_ms"`
	P95Ms int64 `json:"p95_ms"`
}

// ToolLatencyEntry is one row of the per-tool latency histogram. The slice
// returned by ComputeToolHistogram is sorted alphabetically by Tool so the
// markdown render order is stable across runs.
type ToolLatencyEntry struct {
	Tool     string `json:"tool"`
	Calls    int    `json:"calls"`
	Failures int    `json:"failures"`
	P50Ms    int64  `json:"p50_ms"`
	P95Ms    int64  `json:"p95_ms"`
	MaxMs    int64  `json:"max_ms"`
}

// ComputeLatencyStats computes Count/Min/Max/P50/P95 across all events
// using the nearest-rank percentile method (no interpolation).
func ComputeLatencyStats(events []AgentraceEvent) LatencyStats {
	if len(events) == 0 {
		return LatencyStats{}
	}
	durs := make([]int64, 0, len(events))
	for _, e := range events {
		durs = append(durs, e.DurationMS)
	}
	stats := LatencyStats{Count: len(events)}
	stats.MinMs, stats.MaxMs = minMaxInt64(durs)
	stats.P50Ms = percentileInt64(durs, 0.50)
	stats.P95Ms = percentileInt64(durs, 0.95)
	return stats
}

// ComputeToolHistogram groups events by Tool and emits a sorted slice of
// per-tool latency rollups. Events with an empty Tool field are dropped
// because they cannot be attributed.
func ComputeToolHistogram(events []AgentraceEvent) []ToolLatencyEntry {
	byTool := make(map[string][]AgentraceEvent)
	for _, e := range events {
		if e.Tool == "" {
			continue
		}
		byTool[e.Tool] = append(byTool[e.Tool], e)
	}
	if len(byTool) == 0 {
		return nil
	}
	tools := make([]string, 0, len(byTool))
	for t := range byTool {
		tools = append(tools, t)
	}
	sort.Strings(tools)

	out := make([]ToolLatencyEntry, 0, len(tools))
	for _, tool := range tools {
		evs := byTool[tool]
		stats := ComputeLatencyStats(evs)
		failures := 0
		for _, e := range evs {
			if e.Error != "" || (e.Success != nil && !*e.Success) {
				failures++
			}
		}
		out = append(out, ToolLatencyEntry{
			Tool:     tool,
			Calls:    stats.Count,
			Failures: failures,
			P50Ms:    stats.P50Ms,
			P95Ms:    stats.P95Ms,
			MaxMs:    stats.MaxMs,
		})
	}
	return out
}

// percentileInt64 returns the nearest-rank percentile of the given samples.
// `p` is in the range [0,1]. Returns 0 for an empty input.
func percentileInt64(samples []int64, p float64) int64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]int64, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func minMaxInt64(samples []int64) (int64, int64) {
	if len(samples) == 0 {
		return 0, 0
	}
	mn, mx := samples[0], samples[0]
	for _, v := range samples[1:] {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mn, mx
}
