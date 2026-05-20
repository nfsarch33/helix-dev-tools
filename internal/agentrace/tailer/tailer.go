// Package tailer is a poll-based JSONL tailer used by the Agentrace
// observability pipeline. It scans a single events file (default
// ~/.agentrace/events.jsonl), delivers each newly-appended line to a
// handler callback, and gracefully handles file truncation/rotation
// by resetting the read offset whenever the file size shrinks below
// the previously observed offset.
//
// Poll-based design (no fsnotify): keeps the runtime to stdlib only
// and matches the v323-2 cursor-tools auto-rebuild precedent.
package tailer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Handler is invoked once per fully-terminated JSONL line. Returning
// an error halts the tailer.
type Handler func(ctx context.Context, line []byte) error

// Options configures a Tailer.
type Options struct {
	// Path is the file to tail. Required.
	Path string
	// Interval is the polling cadence. Required (>0).
	Interval time.Duration
	// Handler receives each newly-appended JSONL line. Required.
	Handler Handler
}

// Tailer polls a single JSONL file and delivers appended lines to a
// Handler. Tailers are not safe for concurrent Run.
type Tailer struct {
	path     string
	interval time.Duration
	handler  Handler

	offset int64
}

// New validates opts and returns a configured Tailer.
func New(opts Options) (*Tailer, error) {
	if opts.Path == "" {
		return nil, errors.New("tailer: Options.Path is required")
	}
	if opts.Handler == nil {
		return nil, errors.New("tailer: Options.Handler is required")
	}
	if opts.Interval <= 0 {
		return nil, errors.New("tailer: Options.Interval must be > 0")
	}
	return &Tailer{
		path:     opts.Path,
		interval: opts.Interval,
		handler:  opts.Handler,
	}, nil
}

// Run blocks until ctx is canceled or the handler returns an error.
// Missing files are tolerated (the tailer keeps polling); other I/O
// errors are returned.
func (t *Tailer) Run(ctx context.Context) error {
	if err := t.scan(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := t.scan(ctx); err != nil {
				return err
			}
		}
	}
}

// scan opens the events file (if present), seeks to the persisted
// offset, reads and dispatches every fully-terminated line, then
// updates the offset to the current read position. File truncation
// is detected by stat size < persisted offset and resets the offset
// to zero so the next read replays from the start of the rotated
// file.
func (t *Tailer) scan(ctx context.Context) error {
	f, err := os.Open(t.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("tailer: open %s: %w", t.path, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("tailer: stat %s: %w", t.path, err)
	}
	size := info.Size()

	if size < t.offset {
		t.offset = 0
	}
	if size == t.offset {
		return nil
	}

	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return fmt.Errorf("tailer: seek %s: %w", t.path, err)
	}

	reader := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 && line[len(line)-1] == '\n' {
			t.offset += int64(len(line))
			payload := append([]byte(nil), line[:len(line)-1]...)
			if hErr := t.handler(ctx, payload); hErr != nil {
				return hErr
			}
			continue
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tailer: read %s: %w", t.path, err)
		}
	}
}
