// Package ndjsonutil provides the canonical, race-safe NDJSON writer and
// reader for the helix-dev-tools family of repos. It supersedes per-repo
// internal/ndjson packages with a rotating writer, atomic writes, and a
// streaming tailer.
//
// Migration: sprintboard-mcp and helixon-ec should replace their internal
// copies with an import of this package once their release cadences allow.
package ndjsonutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DefaultMaxBytes = 50 * 1024 * 1024 // 50 MiB default rotation threshold
)

// WriterOption configures a RotatingNDJSONWriter.
type WriterOption func(*RotatingNDJSONWriter)

// WithMaxBytes sets the maximum file size before rotation. Zero disables rotation.
func WithMaxBytes(n int64) WriterOption {
	return func(w *RotatingNDJSONWriter) { w.maxBytes = n }
}

// WithRotateFunc overrides the default rotation naming strategy.
// The function receives the base path and a monotonic sequence number.
func WithRotateFunc(fn func(basePath string, seq int) string) WriterOption {
	return func(w *RotatingNDJSONWriter) { w.rotateFn = fn }
}

// RotatingNDJSONWriter appends JSON lines to a file with optional size-based
// rotation. A nil *RotatingNDJSONWriter is a valid no-op.
type RotatingNDJSONWriter struct {
	mu       sync.Mutex
	f        *os.File
	path     string
	size     int64
	maxBytes int64
	seq      int
	rotateFn func(string, int) string
}

// Open creates or appends to the NDJSON file at path with the given options.
// Parent directories are created with 0o755. An empty path returns nil (no-op).
func Open(path string, opts ...WriterOption) (*RotatingNDJSONWriter, error) {
	if path == "" {
		return nil, nil
	}
	w := &RotatingNDJSONWriter{
		path:     path,
		maxBytes: DefaultMaxBytes,
		rotateFn: defaultRotateName,
	}
	for _, o := range opts {
		o(w)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("ndjsonutil: mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := w.openFile(); err != nil {
		return nil, err
	}
	return w, nil
}

// NewWriter wraps an existing file (useful in tests or when the file is
// already open). The caller is responsible for closing f separately.
func NewWriter(f *os.File) *RotatingNDJSONWriter {
	if f == nil {
		return nil
	}
	info, _ := f.Stat()
	var sz int64
	if info != nil {
		sz = info.Size()
	}
	return &RotatingNDJSONWriter{f: f, size: sz, maxBytes: DefaultMaxBytes, rotateFn: defaultRotateName}
}

// Path returns the current backing file path.
func (w *RotatingNDJSONWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}

// Append marshals event as JSON and writes one NDJSON line atomically (single
// Write call). Safe on nil receiver.
func (w *RotatingNDJSONWriter) Append(event any) error {
	if w == nil {
		return nil
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("ndjsonutil: marshal: %w", err)
	}
	line := append(data, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.f == nil {
		return fmt.Errorf("ndjsonutil: writer closed")
	}
	if w.maxBytes > 0 && w.size+int64(len(line)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return fmt.Errorf("ndjsonutil: rotate: %w", err)
		}
	}
	n, err := w.f.Write(line)
	w.size += int64(n)
	if err != nil {
		return fmt.Errorf("ndjsonutil: write: %w", err)
	}
	return nil
}

// AppendOne opens the file, appends a single event, and closes. Convenience
// for fire-and-forget logging.
func AppendOne(path string, event any) error {
	w, err := Open(path, WithMaxBytes(0))
	if err != nil {
		return err
	}
	if w == nil {
		return nil
	}
	defer w.Close()
	return w.Append(event)
}

// Close flushes and closes the backing file. Safe on nil and idempotent.
func (w *RotatingNDJSONWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

func (w *RotatingNDJSONWriter) openFile() error {
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("ndjsonutil: open %s: %w", w.path, err)
	}
	info, _ := f.Stat()
	w.f = f
	if info != nil {
		w.size = info.Size()
	}
	return nil
}

func (w *RotatingNDJSONWriter) rotate() error {
	if w.f != nil {
		_ = w.f.Close()
	}
	w.seq++
	rotated := w.rotateFn(w.path, w.seq)
	if err := os.Rename(w.path, rotated); err != nil {
		return err
	}
	w.size = 0
	return w.openFile()
}

func defaultRotateName(base string, seq int) string {
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]
	ts := time.Now().UTC().Format("20060102T150405Z")
	return fmt.Sprintf("%s.%s.%d%s", stem, ts, seq, ext)
}
