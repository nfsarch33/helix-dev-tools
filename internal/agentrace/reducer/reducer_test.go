package reducer

import "testing"

func TestApplyEvent_PreToolUseCreatesAgent(t *testing.T) {
	state := EmptyState()
	event := Event{
		Type:       EventPreToolUse,
		Timestamp:  100,
		SessionID:  SessionID("session-1"),
		AgentID:    AgentID("agent-1"),
		ToolCallID: ToolCallID("tool-1"),
		ToolName:   "Shell",
	}

	got := ApplyEvent(state, event)

	if _, ok := got.Sessions[event.SessionID]; !ok {
		t.Fatalf("session %q was not created", event.SessionID)
	}
	agent, ok := got.Agents[event.AgentID]
	if !ok {
		t.Fatalf("agent %q was not created", event.AgentID)
	}
	if agent.Status != AgentStatusRunning {
		t.Fatalf("agent status = %q, want %q", agent.Status, AgentStatusRunning)
	}
	call, ok := got.ToolCalls[event.ToolCallID]
	if !ok {
		t.Fatalf("tool call %q was not created", event.ToolCallID)
	}
	if call.AgentID != event.AgentID || call.ToolName != event.ToolName || call.StartedAt != event.Timestamp {
		t.Fatalf("tool call = %+v, want agent=%q tool=%q started=%d", call, event.AgentID, event.ToolName, event.Timestamp)
	}
}

func TestApplyEvent_PostToolUseClosesToolCall(t *testing.T) {
	state := ApplyEvent(EmptyState(), Event{
		Type:       EventPreToolUse,
		Timestamp:  100,
		SessionID:  SessionID("session-1"),
		AgentID:    AgentID("agent-1"),
		ToolCallID: ToolCallID("tool-1"),
		ToolName:   "ReadFile",
	})

	got := ApplyEvent(state, Event{
		Type:       EventPostToolUse,
		Timestamp:  140,
		SessionID:  SessionID("session-1"),
		AgentID:    AgentID("agent-1"),
		ToolCallID: ToolCallID("tool-1"),
		Output:     "ok",
		CostUSD:    0.003,
	})

	call := got.ToolCalls[ToolCallID("tool-1")]
	if call.Status != ToolCallStatusSucceeded {
		t.Fatalf("tool call status = %q, want %q", call.Status, ToolCallStatusSucceeded)
	}
	if call.EndedAt == nil || *call.EndedAt != 140 {
		t.Fatalf("tool call ended_at = %v, want 140", call.EndedAt)
	}
	if call.DurationMS != 40 {
		t.Fatalf("tool call duration = %d, want 40", call.DurationMS)
	}
	if call.Output != "ok" || call.CostUSD != 0.003 {
		t.Fatalf("tool call output/cost = %q/%f, want ok/0.003", call.Output, call.CostUSD)
	}
}

func TestApplyEvent_SubagentStartParentChildLink(t *testing.T) {
	got := ApplyEvent(EmptyState(), Event{
		Type:          EventSubagentStart,
		Timestamp:     120,
		SessionID:     SessionID("session-1"),
		ParentAgentID: AgentID("parent"),
		AgentID:       AgentID("child"),
	})

	parent := got.Agents[AgentID("parent")]
	child := got.Agents[AgentID("child")]
	if child.ParentID == nil || *child.ParentID != AgentID("parent") {
		t.Fatalf("child parent = %v, want parent", child.ParentID)
	}
	if len(parent.ChildIDs) != 1 || parent.ChildIDs[0] != AgentID("child") {
		t.Fatalf("parent children = %v, want [child]", parent.ChildIDs)
	}
	if len(got.Edges) != 1 || got.Edges[0].FromAgentID != AgentID("parent") || got.Edges[0].ToAgentID != AgentID("child") {
		t.Fatalf("edges = %+v, want parent->child", got.Edges)
	}
}

func TestApplyEvent_StopSessionEnds(t *testing.T) {
	state := ApplyEvent(EmptyState(), Event{
		Type:      EventUserPromptSubmit,
		Timestamp: 90,
		SessionID: SessionID("session-1"),
		AgentID:   AgentID("agent-1"),
	})

	got := ApplyEvent(state, Event{
		Type:      EventStop,
		Timestamp: 200,
		SessionID: SessionID("session-1"),
		AgentID:   AgentID("agent-1"),
	})

	session := got.Sessions[SessionID("session-1")]
	if session.EndedAt == nil || *session.EndedAt != 200 {
		t.Fatalf("session ended_at = %v, want 200", session.EndedAt)
	}
	agent := got.Agents[AgentID("agent-1")]
	if agent.Status != AgentStatusStopped {
		t.Fatalf("agent status = %q, want %q", agent.Status, AgentStatusStopped)
	}
}

func TestReplayToTimestamp_DeterministicFromFixedSeed(t *testing.T) {
	events := []Event{
		{Type: EventUserPromptSubmit, Timestamp: 10, SessionID: SessionID("session-1"), AgentID: AgentID("agent-1")},
		{Type: EventPreToolUse, Timestamp: 20, SessionID: SessionID("session-1"), AgentID: AgentID("agent-1"), ToolCallID: ToolCallID("tool-1"), ToolName: "Shell"},
		{Type: EventPostToolUse, Timestamp: 40, SessionID: SessionID("session-1"), AgentID: AgentID("agent-1"), ToolCallID: ToolCallID("tool-1")},
		{Type: EventStop, Timestamp: 70, SessionID: SessionID("session-1"), AgentID: AgentID("agent-1")},
	}

	first := ReplayToTimestamp(events, 50)
	second := ReplayToTimestamp(events, 50)

	if first.LastTimestamp != second.LastTimestamp {
		t.Fatalf("last timestamp mismatch: %d != %d", first.LastTimestamp, second.LastTimestamp)
	}
	if first.Sessions[SessionID("session-1")].EndedAt != nil {
		t.Fatalf("session ended before replay timestamp")
	}
	if second.ToolCalls[ToolCallID("tool-1")].Status != ToolCallStatusSucceeded {
		t.Fatalf("tool status = %q, want %q", second.ToolCalls[ToolCallID("tool-1")].Status, ToolCallStatusSucceeded)
	}
}

func TestApplyEvent_DoesNotMutateInput(t *testing.T) {
	state := ApplyEvent(EmptyState(), Event{
		Type:       EventPreToolUse,
		Timestamp:  100,
		SessionID:  SessionID("session-1"),
		AgentID:    AgentID("agent-1"),
		ToolCallID: ToolCallID("tool-1"),
		ToolName:   "Shell",
	})

	originalCall := state.ToolCalls[ToolCallID("tool-1")]
	_ = ApplyEvent(state, Event{
		Type:       EventPostToolUse,
		Timestamp:  130,
		SessionID:  SessionID("session-1"),
		AgentID:    AgentID("agent-1"),
		ToolCallID: ToolCallID("tool-1"),
	})

	afterCall := state.ToolCalls[ToolCallID("tool-1")]
	if afterCall.Status != originalCall.Status || afterCall.EndedAt != nil {
		t.Fatalf("input state mutated: before=%+v after=%+v", originalCall, afterCall)
	}
}
