package mem0persist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake Mem0Writer ---

type fakeMem0Writer struct {
	written []Entry
	failOn  map[string]bool // text -> should fail
}

func (f *fakeMem0Writer) Write(_ context.Context, e Entry) error {
	if f.failOn != nil && f.failOn[e.Text] {
		return errors.New("mem0 unreachable")
	}
	f.written = append(f.written, e)
	return nil
}

// switchableMem0Writer is a thread-safe Mem0Writer that can toggle
// between down (all writes fail) and up (all writes succeed).
type switchableMem0Writer struct {
	mu      sync.Mutex
	down    bool
	written []Entry
}

func (sw *switchableMem0Writer) Write(_ context.Context, e Entry) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.down {
		return errors.New("mem0 unreachable")
	}
	sw.written = append(sw.written, e)
	return nil
}

func (sw *switchableMem0Writer) setDown(d bool) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.down = d
}

func (sw *switchableMem0Writer) count() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return len(sw.written)
}

// --- helpers ---

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "memories", "pending.ndjson")
	s, err := NewStore(path, DefaultMaxFileBytes)
	require.NoError(t, err)
	return s
}

// --- tests ---

func TestPersist_WritesToFile(t *testing.T) {
	s := newTestStore(t)
	e := Entry{
		Text:   "test memory",
		UserID: "nfsarch33",
		AppID:  "cursor-global-kb",
	}

	require.NoError(t, s.Persist(e))

	entries, err := s.ReadAll()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "test memory", entries[0].Text)
	assert.Equal(t, "nfsarch33", entries[0].UserID)
	assert.Equal(t, "cursor-global-kb", entries[0].AppID)
	assert.Equal(t, "git-kb-fallback", entries[0].Source)
	assert.False(t, entries[0].Timestamp.IsZero())
}

func TestPersist_NDJSONFormat(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 3; i++ {
		require.NoError(t, s.Persist(Entry{
			Text:   "memory " + strings.Repeat("x", i),
			UserID: "u1",
			AppID:  "a1",
		}))
	}

	raw, err := os.ReadFile(s.Path())
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	assert.Len(t, lines, 3)
	for _, line := range lines {
		var e Entry
		require.NoError(t, json.Unmarshal([]byte(line), &e), "each line must be valid JSON")
	}
}

func TestPersist_DefaultSource(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Persist(Entry{Text: "t", UserID: "u", AppID: "a"}))

	entries, err := s.ReadAll()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "git-kb-fallback", entries[0].Source)
}

func TestPersist_PreservesExplicitSource(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Persist(Entry{
		Text: "t", UserID: "u", AppID: "a", Source: "custom",
	}))

	entries, err := s.ReadAll()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "custom", entries[0].Source)
}

func TestPersist_SetsTimestamp(t *testing.T) {
	s := newTestStore(t)
	before := time.Now().Add(-time.Second)
	require.NoError(t, s.Persist(Entry{Text: "t", UserID: "u", AppID: "a"}))
	after := time.Now().Add(time.Second)

	entries, err := s.ReadAll()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Timestamp.After(before))
	assert.True(t, entries[0].Timestamp.Before(after))
}

func TestPersist_PreservesExplicitTimestamp(t *testing.T) {
	s := newTestStore(t)
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, s.Persist(Entry{
		Text: "t", UserID: "u", AppID: "a", Timestamp: ts,
	}))

	entries, err := s.ReadAll()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Timestamp.Equal(ts))
}

func TestReconcile_PushesEntriesToMem0(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		require.NoError(t, s.Persist(Entry{
			Text: "mem " + strings.Repeat("x", i), UserID: "u", AppID: "a",
		}))
	}

	w := &fakeMem0Writer{}
	pushed, remaining, err := s.Reconcile(context.Background(), w)
	require.NoError(t, err)
	assert.Equal(t, 5, pushed)
	assert.Equal(t, 0, remaining)
	assert.Len(t, w.written, 5)

	// file should be removed after full drain
	_, statErr := os.Stat(s.Path())
	assert.True(t, errors.Is(statErr, os.ErrNotExist))
}

func TestReconcile_KeepsFailedEntries(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Persist(Entry{Text: "ok1", UserID: "u", AppID: "a"}))
	require.NoError(t, s.Persist(Entry{Text: "fail", UserID: "u", AppID: "a"}))
	require.NoError(t, s.Persist(Entry{Text: "ok2", UserID: "u", AppID: "a"}))

	w := &fakeMem0Writer{failOn: map[string]bool{"fail": true}}
	pushed, remaining, err := s.Reconcile(context.Background(), w)
	require.NoError(t, err)
	assert.Equal(t, 2, pushed)
	assert.Equal(t, 1, remaining)

	entries, err := s.ReadAll()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "fail", entries[0].Text)
}

func TestReconcile_Idempotent(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Persist(Entry{Text: "a", UserID: "u", AppID: "a"}))
	require.NoError(t, s.Persist(Entry{Text: "b", UserID: "u", AppID: "a"}))

	w := &fakeMem0Writer{}

	// first reconcile
	pushed1, rem1, err := s.Reconcile(context.Background(), w)
	require.NoError(t, err)
	assert.Equal(t, 2, pushed1)
	assert.Equal(t, 0, rem1)

	// second reconcile: nothing to push
	pushed2, rem2, err := s.Reconcile(context.Background(), w)
	require.NoError(t, err)
	assert.Equal(t, 0, pushed2)
	assert.Equal(t, 0, rem2)

	// writer should still only have 2 entries
	assert.Len(t, w.written, 2)
}

func TestPersist_FileRotation_RefusesOverMax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pending.ndjson")
	s, err := NewStore(path, 100) // 100 bytes max
	require.NoError(t, err)

	// write enough to exceed limit
	bigEntry := Entry{Text: strings.Repeat("x", 80), UserID: "u", AppID: "a"}
	require.NoError(t, s.Persist(bigEntry))

	// second write should fail: file is now > 100 bytes
	err = s.Persist(bigEntry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rotate before writing")
}

func TestReadAll_EmptyFile(t *testing.T) {
	s := newTestStore(t)
	entries, err := s.ReadAll()
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestReadAll_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.ndjson")
	s, err := NewStore(path, DefaultMaxFileBytes)
	require.NoError(t, err)

	entries, err := s.ReadAll()
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestFileSize(t *testing.T) {
	s := newTestStore(t)

	size, err := s.FileSize()
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)

	require.NoError(t, s.Persist(Entry{Text: "hello", UserID: "u", AppID: "a"}))

	size, err = s.FileSize()
	require.NoError(t, err)
	assert.Greater(t, size, int64(0))
}

func TestNewStore_DefaultPath(t *testing.T) {
	// Just verify it doesn't error with empty path. We can't test
	// the actual path without writing to $HOME, so we only check
	// that the returned store has a non-empty path.
	s, err := NewStore("", DefaultMaxFileBytes)
	if err != nil {
		t.Skipf("skipping default path test: %v", err)
	}
	assert.NotEmpty(t, s.Path())
	assert.Contains(t, s.Path(), "pending.ndjson")
}

func TestConcurrentPersist(t *testing.T) {
	s := newTestStore(t)
	const n = 50
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			errs <- s.Persist(Entry{
				Text: "concurrent", UserID: "u", AppID: "a",
			})
		}(i)
	}
	for i := 0; i < n; i++ {
		require.NoError(t, <-errs)
	}

	entries, err := s.ReadAll()
	require.NoError(t, err)
	assert.Len(t, entries, n)
}

// ---- v415-2 tests ----

func TestPersist_RoundTrip_MemDown_Fallback_Reconcile(t *testing.T) {
	s := newTestStore(t)
	w := &switchableMem0Writer{down: true}

	const total = 10
	for i := 0; i < total; i++ {
		// Mem0 is down → caller falls back to local persistence.
		require.Error(t, w.Write(context.Background(), Entry{
			Text: fmt.Sprintf("mem-%d", i), UserID: "u", AppID: "a",
		}))
		require.NoError(t, s.Persist(Entry{
			Text: fmt.Sprintf("mem-%d", i), UserID: "u", AppID: "a",
		}))
	}

	entries, err := s.ReadAll()
	require.NoError(t, err)
	require.Len(t, entries, total, "all 10 entries should land in the pending file")

	// Bring Mem0 back up and reconcile.
	w.setDown(false)
	pushed, remaining, err := s.Reconcile(context.Background(), w)
	require.NoError(t, err)
	assert.Equal(t, total, pushed)
	assert.Equal(t, 0, remaining)
	assert.Equal(t, total, w.count(), "all 10 entries pushed to Mem0")

	_, statErr := os.Stat(s.Path())
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "pending file should be removed after full drain")
}

func TestPersist_Reconcile_Idempotent(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		require.NoError(t, s.Persist(Entry{
			Text: fmt.Sprintf("entry-%d", i), UserID: "u", AppID: "a",
		}))
	}

	w := &fakeMem0Writer{}

	pushed1, rem1, err := s.Reconcile(context.Background(), w)
	require.NoError(t, err)
	assert.Equal(t, 5, pushed1)
	assert.Equal(t, 0, rem1)

	pushed2, rem2, err := s.Reconcile(context.Background(), w)
	require.NoError(t, err)
	assert.Equal(t, 0, pushed2, "second reconcile should be a no-op")
	assert.Equal(t, 0, rem2)

	assert.Len(t, w.written, 5, "no duplicates in Mem0")
}

func TestPersist_FileRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pending.ndjson")
	maxBytes := int64(512)
	s, err := NewStore(path, maxBytes)
	require.NoError(t, err)

	var preRotationCount int
	for i := 0; ; i++ {
		err := s.Persist(Entry{
			Text:   fmt.Sprintf("entry-%04d", i),
			UserID: "u",
			AppID:  "a",
		})
		if err != nil {
			assert.Contains(t, err.Error(), "rotate before writing")
			break
		}
		preRotationCount++
	}
	require.Greater(t, preRotationCount, 0)

	size, err := s.FileSize()
	require.NoError(t, err)
	require.GreaterOrEqual(t, size, maxBytes)

	archived, err := s.Rotate()
	require.NoError(t, err)
	require.NotEmpty(t, archived)

	_, err = os.Stat(archived)
	require.NoError(t, err, "archived file should exist")

	_, err = os.Stat(s.Path())
	require.True(t, errors.Is(err, os.ErrNotExist), "original path should be gone after rotation")

	const postRotationCount = 3
	for i := 0; i < postRotationCount; i++ {
		require.NoError(t, s.Persist(Entry{
			Text:   fmt.Sprintf("post-rotation-%d", i),
			UserID: "u",
			AppID:  "a",
		}))
	}

	archivedRaw, err := os.ReadFile(archived)
	require.NoError(t, err)
	archivedLines := strings.Split(strings.TrimSpace(string(archivedRaw)), "\n")
	archivedCount := 0
	for _, line := range archivedLines {
		if strings.TrimSpace(line) != "" {
			var e Entry
			require.NoError(t, json.Unmarshal([]byte(line), &e), "archived line must be valid JSON")
			archivedCount++
		}
	}
	assert.Equal(t, preRotationCount, archivedCount, "all pre-rotation entries in archive")

	currentEntries, err := s.ReadAll()
	require.NoError(t, err)
	assert.Len(t, currentEntries, postRotationCount, "post-rotation entries in new file")

	assert.Equal(t, preRotationCount+postRotationCount, archivedCount+len(currentEntries),
		"no data loss across rotation boundary")
}

func TestPersist_ConcurrentWriteAndReconcile(t *testing.T) {
	s := newTestStore(t)
	w := &switchableMem0Writer{}

	const writers = 10
	const entriesPerWriter = 10
	total := writers * entriesPerWriter

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < entriesPerWriter; j++ {
				_ = s.Persist(Entry{
					Text:   fmt.Sprintf("w%d-e%d", id, j),
					UserID: "u",
					AppID:  "a",
				})
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			s.Reconcile(context.Background(), w)
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	// Final reconcile drains whatever remains.
	_, remaining, err := s.Reconcile(context.Background(), w)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)

	leftover, err := s.ReadAll()
	require.NoError(t, err)

	assert.Equal(t, total, w.count()+len(leftover),
		"no data loss: every entry is either in Mem0 or still pending")
}
