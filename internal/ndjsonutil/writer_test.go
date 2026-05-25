package ndjsonutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestOpen_AppendRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "events.ndjson")
	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := w.Append(map[string]any{"ev": "ok", "n": 1}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `"ev":"ok"`) {
		t.Fatalf("missing event: %q", body)
	}
}

func TestOpen_EmptyPathReturnsNil(t *testing.T) {
	t.Parallel()
	w, err := Open("")
	if err != nil {
		t.Fatalf("Open(\"\"): %v", err)
	}
	if w != nil {
		t.Fatalf("expected nil writer for empty path")
	}
	if err := w.Append(map[string]any{}); err != nil {
		t.Fatalf("nil Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}

func TestAppend_AtomicWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.ndjson")
	w, err := Open(path, WithMaxBytes(0))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer w.Close()

	type row struct {
		ID  int    `json:"id"`
		Msg string `json:"msg"`
	}
	for i := 0; i < 100; i++ {
		if err := w.Append(row{ID: i, Msg: "hello"}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	w.Close()

	body, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 100 {
		t.Fatalf("got %d lines, want 100", len(lines))
	}
	for i, line := range lines {
		var r row
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("line %d unmarshal: %v", i, err)
		}
		if r.ID != i {
			t.Fatalf("line %d: ID=%d, want %d", i, r.ID, i)
		}
	}
}

func TestAppend_RaceSafe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "race.ndjson")
	w, err := Open(path, WithMaxBytes(0))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	const goroutines, writes = 16, 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writes; j++ {
				if err := w.Append(map[string]any{"g": id, "j": j}); err != nil {
					t.Errorf("Append: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()
	w.Close()

	body, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if want := goroutines * writes; len(lines) != want {
		t.Fatalf("got %d lines, want %d", len(lines), want)
	}
	for i, line := range lines {
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("line %d not valid JSON: %v -- %q", i, err, line)
		}
	}
}

func TestRotation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "rotate.ndjson")

	w, err := Open(path, WithMaxBytes(100))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	for i := 0; i < 20; i++ {
		if err := w.Append(map[string]any{"i": i, "payload": "abcdefghijklmnop"}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	w.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected rotation to produce >1 file, got %d", len(entries))
	}

	totalLines := 0
	for _, e := range entries {
		body, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
		for _, l := range lines {
			if l != "" {
				totalLines++
			}
		}
	}
	if totalLines != 20 {
		t.Fatalf("total lines across rotated files = %d, want 20", totalLines)
	}
}

func TestAppendOne(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "one.ndjson")
	if err := AppendOne(path, map[string]any{"x": 1}); err != nil {
		t.Fatalf("AppendOne: %v", err)
	}
	if err := AppendOne(path, map[string]any{"x": 2}); err != nil {
		t.Fatalf("AppendOne: %v", err)
	}
	body, _ := os.ReadFile(path)
	if cnt := strings.Count(string(body), "\n"); cnt != 2 {
		t.Fatalf("newline count = %d, want 2", cnt)
	}
}

func TestAppendOne_EmptyPath(t *testing.T) {
	t.Parallel()
	if err := AppendOne("", map[string]any{}); err != nil {
		t.Fatalf("AppendOne(\"\"): %v", err)
	}
}

func TestClose_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "idem.ndjson")
	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestNewWriter_Nil(t *testing.T) {
	t.Parallel()
	w := NewWriter(nil)
	if w != nil {
		t.Fatalf("NewWriter(nil) = %v, want nil", w)
	}
}

func TestPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.ndjson")
	w, _ := Open(path)
	defer w.Close()
	if got := w.Path(); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
	var nilW *RotatingNDJSONWriter
	if got := nilW.Path(); got != "" {
		t.Fatalf("nil.Path() = %q, want empty", got)
	}
}
