package fleetagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NDJSONEntry is a single line in the fleet agent execution log.
type NDJSONEntry struct {
	Timestamp  time.Time `json:"ts"`
	AgentID    string    `json:"agent_id"`
	TicketID   string    `json:"ticket_id"`
	TaskType   string    `json:"task_type,omitempty"`
	Success    bool      `json:"success"`
	DurationMS int64     `json:"duration_ms"`
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// NDJSONReporter writes execution results as NDJSON to a file.
type NDJSONReporter struct {
	path    string
	agentID string
	mu      sync.Mutex
}

// NewNDJSONReporter creates a reporter that appends to the given file.
func NewNDJSONReporter(path, agentID string) (*NDJSONReporter, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create ndjson dir: %w", err)
	}
	return &NDJSONReporter{path: path, agentID: agentID}, nil
}

// Report appends an execution result as a single NDJSON line.
func (r *NDJSONReporter) Report(_ context.Context, result ExecutionResult) error {
	entry := NDJSONEntry{
		Timestamp:  result.Timestamp,
		AgentID:    r.agentID,
		TicketID:   result.TicketID,
		Success:    result.Success,
		DurationMS: result.Duration.Milliseconds(),
	}
	if result.Success {
		entry.Output = truncate(result.Output, 500)
	} else {
		entry.Error = result.Error
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal ndjson: %w", err)
	}
	line = append(line, '\n')

	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open ndjson file: %w", err)
	}
	defer f.Close()

	_, err = f.Write(line)
	return err
}

// MultiReporter fans out reports to multiple reporters.
type MultiReporter struct {
	reporters []Reporter
}

// NewMultiReporter creates a reporter that delegates to all given reporters.
func NewMultiReporter(reporters ...Reporter) *MultiReporter {
	return &MultiReporter{reporters: reporters}
}

// Report sends the result to all child reporters. Returns the first error encountered.
func (m *MultiReporter) Report(ctx context.Context, result ExecutionResult) error {
	var firstErr error
	for _, r := range m.reporters {
		if err := r.Report(ctx, result); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
