package mem0cache

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestUsageTracker_NDJSONFormat(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.ndjson")

	u, err := NewUsageTracker(UsageConfig{LogPath: logPath})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	u.LogAdd(false)
	u.LogSearch(true)
	u.LogFlush(5)
	u.LogRateLimit("add")
	u.Close()

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	expectedEvents := []string{"mem0_add", "mem0_search", "mem0_batch_flush", "mem0_rate_limited"}

	for i, expected := range expectedEvents {
		if !scanner.Scan() {
			t.Fatalf("expected line %d (%s), got EOF", i, expected)
		}
		var ev UsageEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			t.Fatalf("line %d invalid JSON: %v", i, err)
		}
		if ev.Event != expected {
			t.Fatalf("line %d: expected event %s, got %s", i, expected, ev.Event)
		}
		if ev.Timestamp == "" {
			t.Fatalf("line %d: missing timestamp", i)
		}
	}
}

func TestUsageTracker_FileAppend(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "append.ndjson")

	u1, err := NewUsageTracker(UsageConfig{LogPath: logPath})
	if err != nil {
		t.Fatalf("open u1: %v", err)
	}
	u1.LogAdd(false)
	u1.Close()

	u2, err := NewUsageTracker(UsageConfig{LogPath: logPath})
	if err != nil {
		t.Fatalf("open u2: %v", err)
	}
	u2.LogSearch(true)
	u2.Close()

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	lines := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines++
	}
	if lines != 2 {
		t.Fatalf("expected 2 lines after append, got %d", lines)
	}
}

func TestUsageTracker_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "concurrent.ndjson")

	u, err := NewUsageTracker(UsageConfig{LogPath: logPath})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				u.LogAdd(n%4 == 0)
			} else {
				u.LogSearch(n%3 == 0)
			}
		}(i)
	}
	wg.Wait()
	u.Close()

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	lines := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ev UsageEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			t.Fatalf("invalid JSON on line %d: %v", lines, err)
		}
		lines++
	}
	if lines != 50 {
		t.Fatalf("expected 50 lines, got %d", lines)
	}
}

func TestUsageTracker_CreatesDirIfMissing(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "a", "b", "usage.ndjson")

	u, err := NewUsageTracker(UsageConfig{LogPath: logPath})
	if err != nil {
		t.Fatalf("open with nested path: %v", err)
	}
	defer u.Close()

	u.LogAdd(false)

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log file not created: %v", err)
	}
}

func TestUsageTracker_Path(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "path-test.ndjson")

	u, err := NewUsageTracker(UsageConfig{LogPath: logPath})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer u.Close()

	if u.Path() != logPath {
		t.Fatalf("expected path %s, got %s", logPath, u.Path())
	}
}

func TestUsageTracker_DedupMeta(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "meta.ndjson")

	u, err := NewUsageTracker(UsageConfig{LogPath: logPath})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	u.LogAdd(true)
	u.Close()

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan()
	var ev UsageEvent
	if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if ev.Meta["dedup_hit"] != true {
		t.Fatalf("expected dedup_hit=true, got %v", ev.Meta["dedup_hit"])
	}
}
