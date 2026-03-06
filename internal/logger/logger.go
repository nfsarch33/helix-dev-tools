package logger

import (
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
	l.mu.Lock()
	defer l.mu.Unlock()

	ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	line := fmt.Sprintf("[%s] %s\n", ts, msg)

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
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

	ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_ = os.WriteFile(l.path, []byte(fmt.Sprintf("[%s] log rotated\n", ts)), 0o644)
}

// RotateAll rotates multiple log files.
func RotateAll(dir string, names []string) {
	for _, name := range names {
		l := New(filepath.Join(dir, name))
		l.Rotate()
	}
}
