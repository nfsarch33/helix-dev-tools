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

// AppendEvent appends one NDJSON record (creates parent dirs as needed).
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("semble-discipline: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("semble-discipline: open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("semble-discipline: write: %w", err)
	}
	return nil
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
