package agentidentity

import (
	"os"
	"strings"
)

type AgentID string

const (
	CursorParent AgentID = "cursor-parent"
	Codex        AgentID = "codex"
	CodexEC      AgentID = "codex-ec"
	CodexJobhunt AgentID = "codex-jobhunt"
	ClaudeCode   AgentID = "claude-code"
	Operator     AgentID = "operator"
)

type AgentInfo struct {
	ID       AgentID `json:"id"`
	Surface  string  `json:"surface"`
	Session  string  `json:"session_id,omitempty"`
	Hostname string  `json:"hostname,omitempty"`
}

func Resolve() AgentInfo {
	info := AgentInfo{Hostname: hostname()}

	if id := os.Getenv("CURSOR_AGENT_ID"); id != "" {
		info.ID = AgentID(id)
		info.Surface = "cursor"
		return info
	}

	if os.Getenv("CODEX_SESSION") != "" {
		info.ID = Codex
		info.Surface = "codex-cli"
		info.Session = os.Getenv("CODEX_SESSION")
		if profile := os.Getenv("CODEX_PROFILE"); profile != "" {
			info.ID = AgentID("codex-" + strings.ToLower(profile))
		}
		return info
	}

	if os.Getenv("CLAUDE_CODE") != "" || os.Getenv("CLAUDE_SESSION_ID") != "" {
		info.ID = ClaudeCode
		info.Surface = "claude-code"
		info.Session = os.Getenv("CLAUDE_SESSION_ID")
		return info
	}

	if os.Getenv("CURSOR") != "" || os.Getenv("CURSOR_SESSION_ID") != "" {
		info.ID = CursorParent
		info.Surface = "cursor"
		info.Session = os.Getenv("CURSOR_SESSION_ID")
		return info
	}

	info.ID = Operator
	info.Surface = "terminal"
	return info
}

func IsKnown(id AgentID) bool {
	known := map[AgentID]bool{
		CursorParent: true,
		Codex:        true,
		CodexEC:      true,
		CodexJobhunt: true,
		ClaudeCode:   true,
		Operator:     true,
	}
	return known[id]
}

func AllAgents() []AgentID {
	return []AgentID{CursorParent, Codex, CodexEC, CodexJobhunt, ClaudeCode, Operator}
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}
