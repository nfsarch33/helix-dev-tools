package sprinteval

import (
	"testing"
	"time"
)

// TestV8700_B13_ComputeLatencyStats asserts the P50 / P95 / count surface
// against a known fixed dataset. Strict-acceptance: the eval report can
// report tool latency percentiles, not just averages.
func TestV8700_B13_ComputeLatencyStats(t *testing.T) {
	t.Parallel()

	// 10 events, durations 10..100 ms, all successful.
	events := make([]AgentraceEvent, 0, 10)
	for i := 1; i <= 10; i++ {
		ok := true
		events = append(events, AgentraceEvent{
			Timestamp:  time.Now(),
			Tool:       "memory.search",
			DurationMS: int64(i * 10),
			Success:    &ok,
		})
	}

	stats := ComputeLatencyStats(events)
	if stats.Count != 10 {
		t.Fatalf("count = %d, want 10", stats.Count)
	}
	// Nearest-rank: P50 of 10 sorted samples is index ceil(0.5*10)-1 = 4 -> 50.
	if stats.P50Ms != 50 {
		t.Fatalf("p50 = %d, want 50", stats.P50Ms)
	}
	// P95 of 10 sorted samples is index ceil(0.95*10)-1 = 9 -> 100.
	if stats.P95Ms != 100 {
		t.Fatalf("p95 = %d, want 100", stats.P95Ms)
	}
	if stats.MaxMs != 100 || stats.MinMs != 10 {
		t.Fatalf("min/max = %d/%d, want 10/100", stats.MinMs, stats.MaxMs)
	}
}

// TestV8700_B13_ComputeLatencyStats_EmptyAndZero asserts the no-data and
// all-zero branches don't panic and return a deterministic zero result.
func TestV8700_B13_ComputeLatencyStats_EmptyAndZero(t *testing.T) {
	t.Parallel()

	if got := ComputeLatencyStats(nil); got.Count != 0 || got.P50Ms != 0 || got.P95Ms != 0 {
		t.Fatalf("empty = %+v, want all zero", got)
	}
	// Events with no duration field are still counted against Count but
	// contribute zero to the percentile distribution.
	events := []AgentraceEvent{{Tool: "x", DurationMS: 0}, {Tool: "y", DurationMS: 0}}
	got := ComputeLatencyStats(events)
	if got.Count != 2 || got.P50Ms != 0 || got.P95Ms != 0 {
		t.Fatalf("zero-duration events = %+v, want count=2 p50=0 p95=0", got)
	}
}

// TestV8700_B13_ComputeToolHistogram asserts the per-tool latency rollup
// returns a stable, sorted slice keyed by tool name. Strict-acceptance:
// the eval report can show "tool X had 3 calls, P95 = 42ms, 1 failure".
func TestV8700_B13_ComputeToolHistogram(t *testing.T) {
	t.Parallel()

	ok, fail := true, false
	events := []AgentraceEvent{
		{Tool: "memory.search", DurationMS: 10, Success: &ok},
		{Tool: "memory.search", DurationMS: 30, Success: &ok},
		{Tool: "memory.search", DurationMS: 50, Success: &fail, Error: "boom"},
		{Tool: "sprintboard.claim_ticket", DurationMS: 8, Success: &ok},
		{Tool: "", DurationMS: 999}, // drop: empty tool name
	}

	hist := ComputeToolHistogram(events)
	if len(hist) != 2 {
		t.Fatalf("len = %d, want 2", len(hist))
	}
	// Slice must be sorted alphabetically by tool name for stable output.
	if hist[0].Tool != "memory.search" {
		t.Fatalf("hist[0].Tool = %q, want memory.search", hist[0].Tool)
	}
	if hist[0].Calls != 3 || hist[0].Failures != 1 {
		t.Fatalf("memory.search calls/failures = %d/%d, want 3/1", hist[0].Calls, hist[0].Failures)
	}
	if hist[0].P50Ms != 30 {
		t.Fatalf("memory.search p50 = %d, want 30", hist[0].P50Ms)
	}
	if hist[1].Tool != "sprintboard.claim_ticket" {
		t.Fatalf("hist[1].Tool = %q, want sprintboard.claim_ticket", hist[1].Tool)
	}
}
