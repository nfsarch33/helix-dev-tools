package reducer

func ApplyEvent(state State, event Event) State {
	next := CloneState(state)
	if event.Timestamp > next.LastTimestamp {
		next.LastTimestamp = event.Timestamp
	}

	switch event.Type {
	case EventUserPromptSubmit:
		ensureAgent(&next, event.AgentID, event.SessionID, event.Timestamp)
	case EventPreToolUse:
		applyPreToolUse(&next, event)
	case EventPostToolUse:
		applyPostToolUse(&next, event, ToolCallStatusSucceeded)
	case EventPostToolUseFailure:
		applyPostToolUse(&next, event, ToolCallStatusFailed)
	case EventSubagentStart:
		applySubagentStart(&next, event)
	case EventSubagentStop:
		applySubagentStop(&next, event)
	case EventStop:
		applyStop(&next, event)
	}

	return next
}

func ReplayToTimestamp(events []Event, ts int64) State {
	state := EmptyState()
	for _, event := range events {
		if event.Timestamp <= ts {
			state = ApplyEvent(state, event)
		}
	}
	return state
}

func applyPreToolUse(state *State, event Event) {
	ensureAgent(state, event.AgentID, event.SessionID, event.Timestamp)
	if event.ToolCallID == "" {
		return
	}
	state.ToolCalls[event.ToolCallID] = ToolCall{
		ID:           event.ToolCallID,
		SessionID:    normalizedSessionID(event.SessionID),
		AgentID:      normalizedAgentID(event.AgentID),
		ToolName:     event.ToolName,
		Status:       ToolCallStatusRunning,
		StartedAt:    event.Timestamp,
		CostUSD:      event.CostUSD,
		InputTokens:  event.InputTokens,
		OutputTokens: event.OutputTokens,
	}
	if event.Iteration > 0 {
		state.Iterations = append(state.Iterations, Iteration{
			AgentID:   normalizedAgentID(event.AgentID),
			Number:    event.Iteration,
			Timestamp: event.Timestamp,
			ToolID:    event.ToolCallID,
		})
	}
}

func applyPostToolUse(state *State, event Event, status ToolCallStatus) {
	ensureAgent(state, event.AgentID, event.SessionID, event.Timestamp)
	call, ok := state.ToolCalls[event.ToolCallID]
	if !ok {
		call = ToolCall{
			ID:        event.ToolCallID,
			SessionID: normalizedSessionID(event.SessionID),
			AgentID:   normalizedAgentID(event.AgentID),
			ToolName:  event.ToolName,
			StartedAt: event.Timestamp,
		}
	}
	endedAt := event.Timestamp
	call.EndedAt = &endedAt
	call.DurationMS = endedAt - call.StartedAt
	if call.DurationMS < 0 {
		call.DurationMS = 0
	}
	call.Status = status
	if event.Output != "" {
		call.Output = event.Output
	}
	if event.Error != "" {
		call.Error = event.Error
	}
	if event.CostUSD != 0 {
		call.CostUSD = event.CostUSD
	}
	if event.InputTokens != 0 {
		call.InputTokens = event.InputTokens
	}
	if event.OutputTokens != 0 {
		call.OutputTokens = event.OutputTokens
	}
	state.ToolCalls[event.ToolCallID] = call
}

func applySubagentStart(state *State, event Event) {
	child := ensureAgent(state, event.AgentID, event.SessionID, event.Timestamp)
	parentID := event.ParentAgentID
	if parentID == "" {
		parentID = normalizedAgentID(event.AgentID)
	}
	parent := ensureAgent(state, parentID, event.SessionID, event.Timestamp)
	child.ParentID = &parentID
	child.Status = AgentStatusRunning
	parent = addChild(parent, child.ID)
	state.Agents[child.ID] = child
	state.Agents[parent.ID] = parent
	if !hasEdge(state.Edges, parent.ID, child.ID) {
		state.Edges = append(state.Edges, Edge{FromAgentID: parent.ID, ToAgentID: child.ID, Timestamp: event.Timestamp})
	}
}

func applySubagentStop(state *State, event Event) {
	agent := ensureAgent(state, event.AgentID, event.SessionID, event.Timestamp)
	endedAt := event.Timestamp
	agent.EndedAt = &endedAt
	agent.Status = AgentStatusStopped
	state.Agents[agent.ID] = agent
}

func applyStop(state *State, event Event) {
	sessionID := normalizedSessionID(event.SessionID)
	ensureSession(state, sessionID, event.Timestamp)
	session := state.Sessions[sessionID]
	endedAt := event.Timestamp
	session.EndedAt = &endedAt
	state.Sessions[sessionID] = session

	if event.AgentID != "" {
		agent := ensureAgent(state, event.AgentID, sessionID, event.Timestamp)
		agent.EndedAt = &endedAt
		agent.Status = AgentStatusStopped
		state.Agents[agent.ID] = agent
	}
}

func normalizedSessionID(id SessionID) SessionID {
	if id == "" {
		return SessionID("default")
	}
	return id
}

func normalizedAgentID(id AgentID) AgentID {
	if id == "" {
		return AgentID("root")
	}
	return id
}
