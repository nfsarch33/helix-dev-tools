package insights

import (
	"sort"

	"github.com/nfsarch33/helix-dev-tools/internal/agentrace/reducer"
)

type BottleneckResult struct {
	AgentID    reducer.AgentID    `json:"agent_id"`
	ToolCallID reducer.ToolCallID `json:"tool_call_id"`
	ToolName   string             `json:"tool_name"`
	DurationMS int64              `json:"duration_ms"`
}

func Bottleneck(state reducer.State) (BottleneckResult, bool) {
	var result BottleneckResult
	found := false
	for _, call := range state.ToolCalls {
		duration := call.DurationMS
		if duration == 0 && call.EndedAt != nil {
			duration = *call.EndedAt - call.StartedAt
		}
		if duration < 0 {
			duration = 0
		}
		if !found || duration > result.DurationMS {
			result = BottleneckResult{
				AgentID:    call.AgentID,
				ToolCallID: call.ID,
				ToolName:   call.ToolName,
				DurationMS: duration,
			}
			found = true
		}
	}
	return result, found
}

type Pricing struct {
	InputPerMillion  float64 `json:"input_per_million"`
	OutputPerMillion float64 `json:"output_per_million"`
	ToolFallbackUSD  float64 `json:"tool_fallback_usd"`
}

type CostTierCounts struct {
	Measured       int `json:"measured"`
	TokenEstimated int `json:"token_estimated"`
	ToolFallback   int `json:"tool_fallback"`
}

type CostSummary struct {
	TotalUSD   float64        `json:"total_usd"`
	TierCounts CostTierCounts `json:"tier_counts"`
}

func CostEstimate(state reducer.State, pricing Pricing) CostSummary {
	var summary CostSummary
	for _, call := range state.ToolCalls {
		switch {
		case call.CostUSD > 0:
			summary.TotalUSD += call.CostUSD
			summary.TierCounts.Measured++
		case call.InputTokens > 0 || call.OutputTokens > 0:
			inputCost := float64(call.InputTokens) / 1_000_000 * pricing.InputPerMillion
			outputCost := float64(call.OutputTokens) / 1_000_000 * pricing.OutputPerMillion
			summary.TotalUSD += inputCost + outputCost
			summary.TierCounts.TokenEstimated++
		default:
			summary.TotalUSD += pricing.ToolFallbackUSD
			summary.TierCounts.ToolFallback++
		}
	}
	return summary
}

type ParallelismGap struct {
	ParentAgentID reducer.AgentID `json:"parent_agent_id"`
	FirstAgentID  reducer.AgentID `json:"first_agent_id"`
	SecondAgentID reducer.AgentID `json:"second_agent_id"`
	GapMS         int64           `json:"gap_ms"`
}

func ParallelismGaps(state reducer.State) []ParallelismGap {
	var gaps []ParallelismGap
	for _, parent := range state.Agents {
		if len(parent.ChildIDs) < 2 {
			continue
		}
		children := make([]reducer.Agent, 0, len(parent.ChildIDs))
		for _, childID := range parent.ChildIDs {
			child, ok := state.Agents[childID]
			if ok {
				children = append(children, child)
			}
		}
		sort.Slice(children, func(i, j int) bool {
			if children[i].StartedAt == children[j].StartedAt {
				return children[i].ID < children[j].ID
			}
			return children[i].StartedAt < children[j].StartedAt
		})
		for i := 1; i < len(children); i++ {
			gap := children[i].StartedAt - children[i-1].StartedAt
			if gap < 0 {
				gap = 0
			}
			gaps = append(gaps, ParallelismGap{
				ParentAgentID: parent.ID,
				FirstAgentID:  children[i-1].ID,
				SecondAgentID: children[i].ID,
				GapMS:         gap,
			})
		}
	}
	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].ParentAgentID == gaps[j].ParentAgentID {
			return gaps[i].GapMS > gaps[j].GapMS
		}
		return gaps[i].ParentAgentID < gaps[j].ParentAgentID
	})
	return gaps
}

type StuckSignal struct {
	AgentID     reducer.AgentID `json:"agent_id"`
	ToolName    string          `json:"tool_name"`
	Error       string          `json:"error,omitempty"`
	RepeatCount int             `json:"repeat_count"`
	WindowMS    int64           `json:"window_ms"`
}

func StuckSignals(state reducer.State, windowMS int64) []StuckSignal {
	if windowMS <= 0 {
		windowMS = 60_000
	}
	calls := sortedToolCalls(state)
	var signals []StuckSignal
	for i := 1; i < len(calls); i++ {
		prev := calls[i-1]
		curr := calls[i]
		if prev.Status != reducer.ToolCallStatusFailed || curr.Status != reducer.ToolCallStatusFailed {
			continue
		}
		if prev.AgentID != curr.AgentID || prev.ToolName != curr.ToolName {
			continue
		}
		if curr.StartedAt-prev.StartedAt > windowMS {
			continue
		}
		signals = append(signals, StuckSignal{
			AgentID:     curr.AgentID,
			ToolName:    curr.ToolName,
			Error:       curr.Error,
			RepeatCount: 2,
			WindowMS:    curr.StartedAt - prev.StartedAt,
		})
	}
	return signals
}

type RecoverySignal struct {
	AgentID            reducer.AgentID    `json:"agent_id"`
	FailedToolCallID   reducer.ToolCallID `json:"failed_tool_call_id"`
	RecoveryToolCallID reducer.ToolCallID `json:"recovery_tool_call_id,omitempty"`
	Recovered          bool               `json:"recovered"`
	LatencyMS          int64              `json:"latency_ms,omitempty"`
}

func ErrorRecovery(state reducer.State) []RecoverySignal {
	calls := sortedToolCalls(state)
	var signals []RecoverySignal
	for i, call := range calls {
		if call.Status != reducer.ToolCallStatusFailed {
			continue
		}
		signal := RecoverySignal{AgentID: call.AgentID, FailedToolCallID: call.ID}
		for _, candidate := range calls[i+1:] {
			if candidate.AgentID != call.AgentID {
				continue
			}
			if candidate.Status == reducer.ToolCallStatusSucceeded {
				signal.Recovered = true
				signal.RecoveryToolCallID = candidate.ID
				signal.LatencyMS = candidate.StartedAt - call.StartedAt
				if signal.LatencyMS < 0 {
					signal.LatencyMS = 0
				}
				break
			}
		}
		signals = append(signals, signal)
	}
	return signals
}

type BudgetStatus struct {
	ThresholdUSD float64 `json:"threshold_usd"`
	TotalUSD     float64 `json:"total_usd"`
	OverByUSD    float64 `json:"over_by_usd"`
	Exceeded     bool    `json:"exceeded"`
}

func BudgetExceeded(cost CostSummary, thresholdUSD float64) BudgetStatus {
	status := BudgetStatus{ThresholdUSD: thresholdUSD, TotalUSD: cost.TotalUSD}
	if cost.TotalUSD > thresholdUSD {
		status.Exceeded = true
		status.OverByUSD = cost.TotalUSD - thresholdUSD
	}
	return status
}

func sortedToolCalls(state reducer.State) []reducer.ToolCall {
	calls := make([]reducer.ToolCall, 0, len(state.ToolCalls))
	for _, call := range state.ToolCalls {
		calls = append(calls, call)
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].StartedAt == calls[j].StartedAt {
			return calls[i].ID < calls[j].ID
		}
		return calls[i].StartedAt < calls[j].StartedAt
	})
	return calls
}
