package semblediscipline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event is one NDJSON line in ~/logs/runx/semble-discipline.ndjson.
type Event struct {
	Event     string `json:"event"`
	TS        string `json:"ts"`
	Tool      string `json:"tool,omitempty"`
	Verdict   string `json:"verdict"`
	Reason    string `json:"reason,omitempty"`
	Command   string `json:"command,omitempty"`
	Hook      string `json:"hook,omitempty"`
	Strict    bool   `json:"strict,omitempty"`
}

// DefaultLogPath is the advisory log destination.
func DefaultLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("logs", "runx", "semble-discipline.ndjson")
	}
	return filepath.Join(home, "logs", "runx", "semble-discipline.ndjson")
}

var logMu sync.Mutex

// AgentraceLogPath returns the shared agentrace NDJSON path.
func AgentraceLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("logs", "runx", "agentrace-mcp.ndjson")
	}
	return filepath.Join(home, "logs", "runx", "agentrace-mcp.ndjson")
}

// agentraceEvent is the format compatible with the main agentrace log.
type agentraceEvent struct {
	TS      string `json:"ts"`
	Event   string `json:"event_type"`
	Tool    string `json:"tool"`
	AgentID string `json:"agent_id,omitempty"`
	Success bool   `json:"success"`
	Detail  string `json:"detail,omitempty"`
}

// AppendEvent appends one NDJSON record to both the semble-discipline log
// and the main agentrace log for unified coverage tracking.
func AppendEvent(path string, ev Event) error {
	if path == "" {
		path = DefaultLogPath()
	}
	if ev.Event == "" {
		ev.Event = "semble_discipline"
	}
	if ev.TS == "" {
		ev.TS = time.Now().Format(time.RFC3339)
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("semble-discipline: marshal: %w", err)
	}

	logMu.Lock()
	defer logMu.Unlock()

	if err := appendToFile(path, data); err != nil {
		return err
	}

	atEvent := agentraceEvent{
		TS:      ev.TS,
		Event:   "grep_fallback",
		Tool:    ev.Tool,
		AgentID: detectAgentID(),
		Success: true,
		Detail:  ev.Reason,
	}
	atData, err := json.Marshal(atEvent)
	if err == nil {
		_ = appendToFile(AgentraceLogPath(), atData)
	}
	return nil
}

func appendToFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("semble-discipline: mkdir %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("semble-discipline: open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("semble-discipline: write %s: %w", path, err)
	}
	return nil
}

func detectAgentID() string {
	if os.Getenv("CURSOR") != "" || os.Getenv("CURSOR_SESSION_ID") != "" {
		return "cursor-parent"
	}
	if os.Getenv("CODEX_SESSION") != "" {
		return "codex"
	}
	if os.Getenv("CLAUDE_CODE") != "" {
		return "claude-code"
	}
	return "unknown"
}

// StrictModeEnabled returns true when CURSOR_TOOLS_SEMBLE_STRICT is set.
func StrictModeEnabled() bool {
	v := os.Getenv("CURSOR_TOOLS_SEMBLE_STRICT")
	switch v {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}
