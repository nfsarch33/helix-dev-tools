package outcomes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Default buffered NDJSON path under $HOME/.cache/cursor-tools/.
const (
	defaultBufferedDir   = ".cache/cursor-tools"
	defaultBufferedFile  = "outcomes.ndjson"
	defaultBufferedBytes = 4 * 1024 * 1024 // 4 MiB rotation
	defaultBufferedFiles = 5               // keep last 5 archives
)

// BufferedConfig controls the local NDJSON sink. All fields are optional and
// safe defaults apply.
type BufferedConfig struct {
	// Path is the NDJSON file path. If empty, defaults to
	// $HOME/.cache/cursor-tools/outcomes.ndjson.
	Path string
	// MaxBytes triggers rotation when the file exceeds this size.
	// 0 = use defaultBufferedBytes.
	MaxBytes int64
	// MaxFiles caps the number of rotated archives kept.
	// 0 = use defaultBufferedFiles.
	MaxFiles int
}

// BufferedEmitter writes Outcomes as NDJSON to a local file. It is the default
// hook sink because writes complete in microseconds and never block on the
// network. A separate `cursor-tools outcome flush` job uploads them to Mem0.
type BufferedEmitter struct {
	cfg BufferedConfig
	mu  sync.Mutex
	f   *os.File
}

// NewBufferedEmitter prepares the sink and creates the parent directory.
func NewBufferedEmitter(cfg BufferedConfig) (*BufferedEmitter, error) {
	if cfg.Path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolving home dir: %w", err)
		}
		cfg.Path = filepath.Join(home, defaultBufferedDir, defaultBufferedFile)
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = defaultBufferedBytes
	}
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = defaultBufferedFiles
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("creating dir: %w", err)
	}
	return &BufferedEmitter{cfg: cfg}, nil
}

// Path returns the active NDJSON path.
func (b *BufferedEmitter) Path() string { return b.cfg.Path }

// Emit appends a single Outcome line. Concurrency-safe.
func (b *BufferedEmitter) Emit(_ context.Context, o Outcome) error {
	o.Normalize()
	if err := o.Validate(); err != nil {
		return fmt.Errorf("buffered emit: %w", err)
	}

	data, err := json.Marshal(o)
	if err != nil {
		return fmt.Errorf("marshal outcome: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.maybeRotateLocked(int64(len(data) + 1)); err != nil {
		return err
	}
	if err := b.openLocked(); err != nil {
		return err
	}
	if _, err := b.f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write outcome: %w", err)
	}
	return nil
}

// Flush ensures the underlying file is synced to disk.
func (b *BufferedEmitter) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.f == nil {
		return nil
	}
	return b.f.Sync()
}

// Close flushes and closes the file.
func (b *BufferedEmitter) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.f == nil {
		return nil
	}
	err := b.f.Sync()
	closeErr := b.f.Close()
	b.f = nil
	if err != nil {
		return err
	}
	return closeErr
}

func (b *BufferedEmitter) openLocked() error {
	if b.f != nil {
		return nil
	}
	f, err := os.OpenFile(b.cfg.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ndjson: %w", err)
	}
	b.f = f
	return nil
}

func (b *BufferedEmitter) maybeRotateLocked(addBytes int64) error {
	info, err := os.Stat(b.cfg.Path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat ndjson: %w", err)
	}
	if info.Size()+addBytes <= b.cfg.MaxBytes {
		return nil
	}

	if b.f != nil {
		_ = b.f.Sync()
		_ = b.f.Close()
		b.f = nil
	}

	stamp := time.Now().UTC().Format("20060102T150405.000")
	rotated := fmt.Sprintf("%s.%s", b.cfg.Path, stamp)
	if err := os.Rename(b.cfg.Path, rotated); err != nil {
		return fmt.Errorf("rotate ndjson: %w", err)
	}
	return b.pruneArchivesLocked()
}

func (b *BufferedEmitter) pruneArchivesLocked() error {
	matches, err := filepath.Glob(b.cfg.Path + ".*")
	if err != nil {
		return fmt.Errorf("glob archives: %w", err)
	}
	if len(matches) <= b.cfg.MaxFiles {
		return nil
	}
	sort.Strings(matches)
	for _, p := range matches[:len(matches)-b.cfg.MaxFiles] {
		_ = os.Remove(p)
	}
	return nil
}
