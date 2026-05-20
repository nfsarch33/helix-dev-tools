package reducer

type EventType string

const (
	EventPreToolUse         EventType = "PreToolUse"
	EventPostToolUse        EventType = "PostToolUse"
	EventPostToolUseFailure EventType = "PostToolUseFailure"
	EventSubagentStart      EventType = "SubagentStart"
	EventSubagentStop       EventType = "SubagentStop"
	EventStop               EventType = "Stop"
	EventUserPromptSubmit   EventType = "UserPromptSubmit"
)

type AgentID string
type SessionID string
type ToolCallID string

type AgentStatus string

const (
	AgentStatusRunning AgentStatus = "running"
	AgentStatusStopped AgentStatus = "stopped"
	AgentStatusFailed  AgentStatus = "failed"
)

type ToolCallStatus string

const (
	ToolCallStatusRunning   ToolCallStatus = "running"
	ToolCallStatusSucceeded ToolCallStatus = "succeeded"
	ToolCallStatusFailed    ToolCallStatus = "failed"
)

type Event struct {
	Type          EventType  `json:"type"`
	Timestamp     int64      `json:"timestamp"`
	SessionID     SessionID  `json:"session_id,omitempty"`
	AgentID       AgentID    `json:"agent_id,omitempty"`
	ParentAgentID AgentID    `json:"parent_agent_id,omitempty"`
	ToolCallID    ToolCallID `json:"tool_call_id,omitempty"`
	ToolName      string     `json:"tool_name,omitempty"`
	Prompt        string     `json:"prompt,omitempty"`
	Output        string     `json:"output,omitempty"`
	Error         string     `json:"error,omitempty"`
	CostUSD       float64    `json:"cost_usd,omitempty"`
	InputTokens   int        `json:"input_tokens,omitempty"`
	OutputTokens  int        `json:"output_tokens,omitempty"`
	Iteration     int        `json:"iteration,omitempty"`
	// Payload keeps hook-specific JSON fields that are intentionally open-ended.
	Payload map[string]any `json:"payload,omitempty"`
}

type Agent struct {
	ID        AgentID     `json:"id"`
	SessionID SessionID   `json:"session_id"`
	ParentID  *AgentID    `json:"parent_id,omitempty"`
	ChildIDs  []AgentID   `json:"child_ids,omitempty"`
	Status    AgentStatus `json:"status"`
	StartedAt int64       `json:"started_at"`
	EndedAt   *int64      `json:"ended_at,omitempty"`
}

type Session struct {
	ID        SessionID `json:"id"`
	AgentIDs  []AgentID `json:"agent_ids,omitempty"`
	StartedAt int64     `json:"started_at"`
	EndedAt   *int64    `json:"ended_at,omitempty"`
}

type ToolCall struct {
	ID           ToolCallID     `json:"id"`
	SessionID    SessionID      `json:"session_id"`
	AgentID      AgentID        `json:"agent_id"`
	ToolName     string         `json:"tool_name"`
	Status       ToolCallStatus `json:"status"`
	StartedAt    int64          `json:"started_at"`
	EndedAt      *int64         `json:"ended_at,omitempty"`
	DurationMS   int64          `json:"duration_ms,omitempty"`
	Output       string         `json:"output,omitempty"`
	Error        string         `json:"error,omitempty"`
	CostUSD      float64        `json:"cost_usd,omitempty"`
	InputTokens  int            `json:"input_tokens,omitempty"`
	OutputTokens int            `json:"output_tokens,omitempty"`
}

type Edge struct {
	FromAgentID AgentID `json:"from_agent_id"`
	ToAgentID   AgentID `json:"to_agent_id"`
	Timestamp   int64   `json:"timestamp"`
}

type Iteration struct {
	AgentID   AgentID    `json:"agent_id"`
	Number    int        `json:"number"`
	Timestamp int64      `json:"timestamp"`
	ToolID    ToolCallID `json:"tool_id,omitempty"`
}

type State struct {
	Sessions      map[SessionID]Session   `json:"sessions"`
	Agents        map[AgentID]Agent       `json:"agents"`
	ToolCalls     map[ToolCallID]ToolCall `json:"tool_calls"`
	Edges         []Edge                  `json:"edges"`
	Iterations    []Iteration             `json:"iterations"`
	LastTimestamp int64                   `json:"last_timestamp"`
}
