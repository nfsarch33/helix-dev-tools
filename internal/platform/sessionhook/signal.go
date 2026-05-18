package sessionhook

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SignalType classifies an agent action on a file suggestion.
type SignalType string

const (
	SignalAccept SignalType = "accept"
	SignalReject SignalType = "reject"
	SignalEdit   SignalType = "edit"
	SignalRevert SignalType = "revert"
)

// SignalEvent is an agentrace NDJSON record emitted when an agent
// accepts, rejects, edits, or reverts a file change.
type SignalEvent struct {
	Timestamp string     `json:"ts"`
	AgentID   string     `json:"agent_id"`
	Signal    SignalType `json:"signal"`
	FilePath  string     `json:"file_path,omitempty"`
	SprintID  string     `json:"sprint_id,omitempty"`
	TicketID  string     `json:"ticket_id,omitempty"`
	Note      string     `json:"note,omitempty"`
}

// RecordSignal appends a SignalEvent to the agentrace NDJSON log at logPath.
// If logPath is empty it defaults to ~/logs/runx/agentrace-signals.ndjson.
func RecordSignal(agentID string, sig SignalType, filePath, sprintID, ticketID, note string, logPath string) error {
	if logPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home dir: %w", err)
		}
		logPath = home + "/logs/runx/agentrace-signals.ndjson"
	}

	if err := os.MkdirAll(dirOf(logPath), 0o750); err != nil {
		return fmt.Errorf("mkdir log dir: %w", err)
	}

	ev := SignalEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		AgentID:   agentID,
		Signal:    sig,
		FilePath:  filePath,
		SprintID:  sprintID,
		TicketID:  ticketID,
		Note:      note,
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal signal event: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s\n", line)
	return err
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
