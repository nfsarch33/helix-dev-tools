package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultMaxBytes = 512000
	rotatedSuffix   = ".1"
)

// Logger provides structured, append-only hook logging with rotation.
type Logger struct {
	path     string
	maxBytes int64
	mu       sync.Mutex
}

// Entry is a single structured log event written as JSONL.
type Entry struct {
	Timestamp  time.Time      `json:"ts"`
	Level      string         `json:"level,omitempty"`
	Message    string         `json:"msg,omitempty"`
	Hook       string         `json:"hook,omitempty"`
	Command    string         `json:"command,omitempty"`
	Profile    string         `json:"profile,omitempty"`
	Suite      string         `json:"suite,omitempty"`
	Result     string         `json:"result,omitempty"`
	RunID      string         `json:"run_id,omitempty"`
	DurationMs int64          `json:"duration_ms,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// New creates a logger that writes to the given file path.
func New(path string) *Logger {
	return &Logger{path: path, maxBytes: defaultMaxBytes}
}

// WithMaxBytes sets the rotation threshold.
func (l *Logger) WithMaxBytes(n int64) *Logger {
	l.maxBytes = n
	return l
}

// Log appends a timestamped message to the log file.
func (l *Logger) Log(msg string) {
	l.LogEntry(Entry{
		Level:   "info",
		Message: msg,
	})
}

// LogEntry appends a structured JSONL event to the log file.
func (l *Logger) LogEntry(entry Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	if entry.Level == "" {
		entry.Level = "info"
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

// Rotate checks file size and rotates if necessary.
func (l *Logger) Rotate() {
	l.mu.Lock()
	defer l.mu.Unlock()

	info, err := os.Stat(l.path)
	if err != nil || info.Size() <= l.maxBytes {
		return
	}
	rotated := l.path + rotatedSuffix
	_ = os.Rename(l.path, rotated)
	_ = os.WriteFile(l.path, nil, 0o644)
}

// RotateAll rotates multiple log files.
func RotateAll(dir string, names []string) {
	for _, name := range names {
		l := New(filepath.Join(dir, name))
		l.Rotate()
	}
}
