// Package mem0persist implements a Git-KB-backed NDJSON fallback for
// Mem0 memory entries. When Mem0 OSS is unreachable, callers write
// entries to $HOME/Code/global-kb/memories/pending.ndjson. A separate
// reconcile pass drains the file back into Mem0 when connectivity
// returns.
package mem0persist

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DefaultMaxFileBytes = 10 * 1024 * 1024 // 10 MB
	defaultRelPath      = "Code/global-kb/memories/pending.ndjson"
)

// Entry is a single memory record persisted to the NDJSON fallback.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Text      string    `json:"text"`
	UserID    string    `json:"user_id"`
	AppID     string    `json:"app_id"`
	Source    string    `json:"source"`
}

// Mem0Writer abstracts the Mem0 write path so tests can inject fakes.
type Mem0Writer interface {
	Write(ctx context.Context, e Entry) error
}

// Store handles fallback persistence and reconciliation.
type Store struct {
	mu           sync.Mutex
	path         string
	maxFileBytes int64
}

// NewStore returns a Store that writes to path. If path is empty, the
// default location ($HOME/Code/global-kb/memories/pending.ndjson) is
// used.
func NewStore(path string, maxFileBytes int64) (*Store, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home: %w", err)
		}
		path = filepath.Join(home, defaultRelPath)
	}
	if maxFileBytes <= 0 {
		maxFileBytes = DefaultMaxFileBytes
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir persist dir: %w", err)
	}
	return &Store{path: path, maxFileBytes: maxFileBytes}, nil
}

// Path returns the backing NDJSON file path.
func (s *Store) Path() string { return s.path }

// Persist appends an entry to the NDJSON file. It refuses to write if
// the file already exceeds maxFileBytes (rotation guard).
func (s *Store) Persist(e Entry) error {
	if e.Source == "" {
		e.Source = "git-kb-fallback"
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	info, statErr := os.Stat(s.path)
	if statErr == nil && info.Size() >= s.maxFileBytes {
		return fmt.Errorf("persist file exceeds %d bytes; rotate before writing", s.maxFileBytes)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open persist file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write persist line: %w", err)
	}
	return f.Sync()
}

// ReadAll returns every entry in the NDJSON file.
func (s *Store) ReadAll() ([]Entry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

// Reconcile reads all pending entries, attempts to write each to Mem0
// via w, and rewrites the file with only the entries that failed. The
// operation is idempotent: re-running with the same writer drains
// already-written entries.
func (s *Store) Reconcile(ctx context.Context, w Mem0Writer) (pushed, remaining int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.readAllLocked()
	if err != nil {
		return 0, 0, fmt.Errorf("read pending entries: %w", err)
	}
	if len(entries) == 0 {
		return 0, 0, nil
	}

	var failed []Entry
	for _, e := range entries {
		if err := w.Write(ctx, e); err != nil {
			failed = append(failed, e)
			continue
		}
		pushed++
	}

	if err := s.rewriteLocked(failed); err != nil {
		return pushed, len(failed), fmt.Errorf("rewrite pending: %w", err)
	}
	return pushed, len(failed), nil
}

func (s *Store) readAllLocked() ([]Entry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

func (s *Store) rewriteLocked(entries []Entry) error {
	tmp := s.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, e := range entries {
		line, err := json.Marshal(e)
		if err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
		w.Write(line)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()

	if len(entries) == 0 {
		os.Remove(tmp)
		return os.Remove(s.path)
	}
	return os.Rename(tmp, s.path)
}

// FileSize returns the current size of the NDJSON file in bytes.
// Returns 0 if the file does not exist.
func (s *Store) FileSize() (int64, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return info.Size(), nil
}

// Rotate archives the current NDJSON file by renaming it with a
// nanosecond-precision suffix. After rotation the original path is
// free for new writes. Returns the archive path, or ("", nil) if the
// pending file does not exist.
func (s *Store) Rotate() (archived string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, statErr := os.Stat(s.path); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return "", nil
		}
		return "", statErr
	}

	archived = fmt.Sprintf("%s.%d", s.path, time.Now().UnixNano())
	if err := os.Rename(s.path, archived); err != nil {
		return "", fmt.Errorf("rotate: %w", err)
	}
	return archived, nil
}
