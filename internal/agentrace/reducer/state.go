package reducer

func EmptyState() State {
	return State{
		Sessions:  map[SessionID]Session{},
		Agents:    map[AgentID]Agent{},
		ToolCalls: map[ToolCallID]ToolCall{},
	}
}

func CloneState(state State) State {
	next := EmptyState()
	next.LastTimestamp = state.LastTimestamp
	next.Edges = append([]Edge(nil), state.Edges...)
	next.Iterations = append([]Iteration(nil), state.Iterations...)
	for id, session := range state.Sessions {
		next.Sessions[id] = cloneSession(session)
	}
	for id, agent := range state.Agents {
		next.Agents[id] = cloneAgent(agent)
	}
	for id, call := range state.ToolCalls {
		next.ToolCalls[id] = cloneToolCall(call)
	}
	return next
}

func cloneSession(session Session) Session {
	session.AgentIDs = append([]AgentID(nil), session.AgentIDs...)
	session.EndedAt = cloneInt64Ptr(session.EndedAt)
	return session
}

func cloneAgent(agent Agent) Agent {
	agent.ChildIDs = append([]AgentID(nil), agent.ChildIDs...)
	agent.ParentID = cloneAgentIDPtr(agent.ParentID)
	agent.EndedAt = cloneInt64Ptr(agent.EndedAt)
	return agent
}

func cloneToolCall(call ToolCall) ToolCall {
	call.EndedAt = cloneInt64Ptr(call.EndedAt)
	return call
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneAgentIDPtr(value *AgentID) *AgentID {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func ensureSession(state *State, id SessionID, ts int64) Session {
	if id == "" {
		id = SessionID("default")
	}
	session, ok := state.Sessions[id]
	if !ok {
		session = Session{ID: id, StartedAt: ts}
	}
	if session.StartedAt == 0 || ts < session.StartedAt {
		session.StartedAt = ts
	}
	state.Sessions[id] = session
	return session
}

func ensureAgent(state *State, id AgentID, sessionID SessionID, ts int64) Agent {
	if id == "" {
		id = AgentID("root")
	}
	if sessionID == "" {
		sessionID = SessionID("default")
	}
	ensureSession(state, sessionID, ts)
	agent, ok := state.Agents[id]
	if !ok {
		agent = Agent{ID: id, SessionID: sessionID, Status: AgentStatusRunning, StartedAt: ts}
	} else if agent.Status == "" {
		agent.Status = AgentStatusRunning
	}
	if agent.SessionID == "" {
		agent.SessionID = sessionID
	}
	if agent.StartedAt == 0 || ts < agent.StartedAt {
		agent.StartedAt = ts
	}
	state.Agents[id] = agent
	addSessionAgent(state, sessionID, id)
	return agent
}

func addSessionAgent(state *State, sessionID SessionID, agentID AgentID) {
	session := state.Sessions[sessionID]
	for _, existing := range session.AgentIDs {
		if existing == agentID {
			state.Sessions[sessionID] = session
			return
		}
	}
	session.AgentIDs = append(session.AgentIDs, agentID)
	state.Sessions[sessionID] = session
}

func addChild(agent Agent, childID AgentID) Agent {
	for _, existing := range agent.ChildIDs {
		if existing == childID {
			return agent
		}
	}
	agent.ChildIDs = append(agent.ChildIDs, childID)
	return agent
}

func hasEdge(edges []Edge, from AgentID, to AgentID) bool {
	for _, edge := range edges {
		if edge.FromAgentID == from && edge.ToAgentID == to {
			return true
		}
	}
	return false
}
