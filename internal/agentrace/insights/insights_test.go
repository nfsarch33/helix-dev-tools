package insights

import (
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/agentrace/reducer"
)

func TestBottleneck_FindsLongestRunningAgent(t *testing.T) {
	state := reducer.EmptyState()
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPreToolUse, Timestamp: 100, SessionID: "session-1", AgentID: "fast", ToolCallID: "tool-fast", ToolName: "ReadFile"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPostToolUse, Timestamp: 130, SessionID: "session-1", AgentID: "fast", ToolCallID: "tool-fast"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPreToolUse, Timestamp: 110, SessionID: "session-1", AgentID: "slow", ToolCallID: "tool-slow", ToolName: "Shell"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPostToolUse, Timestamp: 220, SessionID: "session-1", AgentID: "slow", ToolCallID: "tool-slow"})

	got, ok := Bottleneck(state)

	if !ok {
		t.Fatalf("expected bottleneck")
	}
	if got.AgentID != reducer.AgentID("slow") || got.DurationMS != 110 {
		t.Fatalf("bottleneck = %+v, want slow/110", got)
	}
}

func TestCostEstimate_ThreeTierFallback(t *testing.T) {
	state := reducer.EmptyState()
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPreToolUse, Timestamp: 100, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "measured", ToolName: "Shell"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPostToolUse, Timestamp: 110, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "measured", CostUSD: 0.25})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPreToolUse, Timestamp: 120, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "tokens", ToolName: "ReadFile"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPostToolUse, Timestamp: 130, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "tokens", InputTokens: 1000, OutputTokens: 500})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPreToolUse, Timestamp: 140, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "fallback", ToolName: "ListDir"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPostToolUse, Timestamp: 150, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "fallback"})

	got := CostEstimate(state, Pricing{InputPerMillion: 3, OutputPerMillion: 15, ToolFallbackUSD: 0.001})

	want := 0.25 + 0.0105 + 0.001
	if got.TierCounts.Measured != 1 || got.TierCounts.TokenEstimated != 1 || got.TierCounts.ToolFallback != 1 {
		t.Fatalf("tier counts = %+v, want 1/1/1", got.TierCounts)
	}
	if diff := got.TotalUSD - want; diff < -0.0000001 || diff > 0.0000001 {
		t.Fatalf("total usd = %.6f, want %.6f", got.TotalUSD, want)
	}
}

func TestParallelismGaps_SiblingPair(t *testing.T) {
	state := reducer.EmptyState()
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventSubagentStart, Timestamp: 100, SessionID: "session-1", ParentAgentID: "parent", AgentID: "child-a"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventSubagentStart, Timestamp: 150, SessionID: "session-1", ParentAgentID: "parent", AgentID: "child-b"})

	got := ParallelismGaps(state)

	if len(got) != 1 {
		t.Fatalf("parallelism gaps = %+v, want one", got)
	}
	if got[0].ParentAgentID != reducer.AgentID("parent") || got[0].FirstAgentID != reducer.AgentID("child-a") || got[0].SecondAgentID != reducer.AgentID("child-b") {
		t.Fatalf("gap = %+v, want parent child-a child-b", got[0])
	}
	if got[0].GapMS != 50 {
		t.Fatalf("gap = %d, want 50", got[0].GapMS)
	}
}

func TestStuckSignals_RepeatedToolCallWithin60s(t *testing.T) {
	state := reducer.EmptyState()
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPreToolUse, Timestamp: 100_000, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "tool-1", ToolName: "Shell"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPostToolUseFailure, Timestamp: 101_000, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "tool-1", Error: "exit 1"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPreToolUse, Timestamp: 120_000, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "tool-2", ToolName: "Shell"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPostToolUseFailure, Timestamp: 121_000, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "tool-2", Error: "exit 1"})

	got := StuckSignals(state, 60_000)

	if len(got) != 1 {
		t.Fatalf("stuck signals = %+v, want one", got)
	}
	if got[0].AgentID != reducer.AgentID("agent-1") || got[0].ToolName != "Shell" || got[0].RepeatCount != 2 {
		t.Fatalf("signal = %+v, want agent-1/Shell repeat 2", got[0])
	}
}

func TestErrorRecovery_FindsRecoveryAfterFailure(t *testing.T) {
	state := reducer.EmptyState()
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPreToolUse, Timestamp: 100, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "bad", ToolName: "Shell"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPostToolUseFailure, Timestamp: 110, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "bad", Error: "exit 1"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPreToolUse, Timestamp: 120, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "good", ToolName: "Shell"})
	state = reducer.ApplyEvent(state, reducer.Event{Type: reducer.EventPostToolUse, Timestamp: 130, SessionID: "session-1", AgentID: "agent-1", ToolCallID: "good"})

	got := ErrorRecovery(state)

	if len(got) != 1 || !got[0].Recovered || got[0].RecoveryToolCallID != reducer.ToolCallID("good") {
		t.Fatalf("recovery = %+v, want recovered by good", got)
	}
}

func TestBudgetExceeded_FlagsOverThreshold(t *testing.T) {
	cost := CostSummary{TotalUSD: 3.50}

	got := BudgetExceeded(cost, 3.00)

	if !got.Exceeded || got.OverByUSD != 0.50 {
		t.Fatalf("budget = %+v, want exceeded by 0.50", got)
	}
}
