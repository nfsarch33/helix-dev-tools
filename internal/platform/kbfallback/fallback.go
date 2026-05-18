package kbfallback

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// State tracks the current fallback mode
type State int

const (
	StateOnline   State = iota // Mem0 reachable
	StateFallback              // Using Git KB fallback
)

// Event is a coordination event written to the fallback store
type Event struct {
	ID        string            `json:"id"`
	AgentID   string            `json:"agent_id"`
	EventType string            `json:"event_type"`
	Data      map[string]string `json:"data"`
	CreatedAt time.Time         `json:"created_at"`
}

// Fallback manages fallback state and coordination event writes
type Fallback struct {
	mem0URL string
	kbDir   string
	state   State
}

// New creates a Fallback with the given Mem0 URL and Git KB directory
func New(mem0URL, kbDir string) *Fallback {
	return &Fallback{mem0URL: mem0URL, kbDir: kbDir, state: StateOnline}
}

// Probe checks if Mem0 is reachable and updates state
func (f *Fallback) Probe() State {
	resp, err := http.Get(f.mem0URL + "/healthz")
	if err != nil || resp.StatusCode >= 400 {
		f.state = StateFallback
		return StateFallback
	}
	resp.Body.Close()
	f.state = StateOnline
	return StateOnline
}

// State returns the current fallback state
func (f *Fallback) CurrentState() State {
	return f.state
}

// WriteEvent writes a coordination event to the Git KB fallback directory
func (f *Fallback) WriteEvent(e Event) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	dir := filepath.Join(f.kbDir, "coordination-events")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	filename := filepath.Join(dir, fmt.Sprintf("%s-%s.json", e.CreatedAt.Format("2006-01-02T150405"), e.ID))
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(filename, data, 0644)
}

// ReadEvents reads all coordination events from the fallback store
func (f *Fallback) ReadEvents() ([]Event, error) {
	dir := filepath.Join(f.kbDir, "coordination-events")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []Event
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var ev Event
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}
