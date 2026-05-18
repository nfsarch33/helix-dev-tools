package agenttrace

import (
	"sort"
	"time"
)

// Event is one tool call or action recorded in a trace
type Event struct {
	SessionID string
	AgentID   string
	Tool      string
	DurationMS int64
	Error      bool
	Timestamp  time.Time
}

// Stats summarises agent performance for a session
type Stats struct {
	SessionID  string
	ToolCounts map[string]int
	ErrorCount int
	TotalMS    int64
	EventCount int
}

// Tracer records and analyses agent tool usage
type Tracer struct {
	events []Event
}

// NewTracer creates an empty tracer
func NewTracer() *Tracer {
	return &Tracer{}
}

// Record appends an event
func (t *Tracer) Record(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	t.events = append(t.events, e)
}

// Stats returns aggregated statistics for a session
func (t *Tracer) Stats(sessionID string) Stats {
	s := Stats{SessionID: sessionID, ToolCounts: map[string]int{}}
	for _, e := range t.events {
		if e.SessionID != sessionID {
			continue
		}
		s.EventCount++
		s.TotalMS += e.DurationMS
		s.ToolCounts[e.Tool]++
		if e.Error {
			s.ErrorCount++
		}
	}
	return s
}

// TopTools returns the N most-used tools across all sessions, sorted by count
func (t *Tracer) TopTools(n int) []string {
	counts := map[string]int{}
	for _, e := range t.events {
		counts[e.Tool]++
	}
	type entry struct {
		tool  string
		count int
	}
	var sorted []entry
	for tool, count := range counts {
		sorted = append(sorted, entry{tool, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})
	result := make([]string, 0, n)
	for i := 0; i < n && i < len(sorted); i++ {
		result = append(result, sorted[i].tool)
	}
	return result
}

// ErrorRate returns the fraction of events that had errors (0 if no events)
func (t *Tracer) ErrorRate() float64 {
	if len(t.events) == 0 {
		return 0
	}
	errCount := 0
	for _, e := range t.events {
		if e.Error {
			errCount++
		}
	}
	return float64(errCount) / float64(len(t.events))
}
