package ndjsonutil

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// NDJSONReader reads NDJSON files line-by-line, unmarshalling into the
// caller's target type.
type NDJSONReader struct {
	f       *os.File
	scanner *bufio.Scanner
}

// OpenReader opens an NDJSON file for sequential reading.
func OpenReader(path string) (*NDJSONReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ndjsonutil: open reader %s: %w", path, err)
	}
	return &NDJSONReader{f: f, scanner: bufio.NewScanner(f)}, nil
}

// Next advances to the next NDJSON line and unmarshals into dest.
// Returns io.EOF when the file is exhausted.
func (r *NDJSONReader) Next(dest any) error {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return fmt.Errorf("ndjsonutil: scan: %w", err)
		}
		return io.EOF
	}
	line := r.scanner.Bytes()
	if len(line) == 0 {
		return r.Next(dest)
	}
	if err := json.Unmarshal(line, dest); err != nil {
		return fmt.Errorf("ndjsonutil: unmarshal: %w", err)
	}
	return nil
}

// ReadAll reads all records into a slice. T must be a struct or map type.
func ReadAll[T any](path string) ([]T, error) {
	r, err := OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var results []T
	for {
		var v T
		if err := r.Next(&v); err == io.EOF {
			break
		} else if err != nil {
			return results, err
		}
		results = append(results, v)
	}
	return results, nil
}

// Close releases the file handle.
func (r *NDJSONReader) Close() error {
	if r == nil || r.f == nil {
		return nil
	}
	return r.f.Close()
}

// Tailer follows an NDJSON file, emitting new lines as they appear (similar
// to tail -f). It respects context cancellation.
type Tailer struct {
	path     string
	f        *os.File
	pollWait time.Duration
}

// TailerOption configures a Tailer.
type TailerOption func(*Tailer)

// WithPollInterval sets the poll frequency for new data (default 200ms).
func WithPollInterval(d time.Duration) TailerOption {
	return func(t *Tailer) { t.pollWait = d }
}

// NewTailer creates a tailer starting from the current end of file.
// Use offset 0 to start from beginning, or -1 for end (default).
func NewTailer(path string, opts ...TailerOption) (*Tailer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ndjsonutil: tailer open %s: %w", path, err)
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		f.Close()
		return nil, fmt.Errorf("ndjsonutil: tailer seek: %w", err)
	}
	t := &Tailer{path: path, f: f, pollWait: 200 * time.Millisecond}
	for _, o := range opts {
		o(t)
	}
	return t, nil
}

// Follow emits each new JSON line to the callback until ctx is cancelled.
// Empty lines are skipped. The callback receives raw JSON bytes.
func (t *Tailer) Follow(ctx context.Context, fn func(line []byte) error) error {
	scanner := bufio.NewScanner(t.f)
	for {
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			if err := fn(line); err != nil {
				return err
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("ndjsonutil: tail scan: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(t.pollWait):
		}
		scanner = bufio.NewScanner(t.f)
	}
}

// Close releases the tailer's file handle.
func (t *Tailer) Close() error {
	if t == nil || t.f == nil {
		return nil
	}
	return t.f.Close()
}
