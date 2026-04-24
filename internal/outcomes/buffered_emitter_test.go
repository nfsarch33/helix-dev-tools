package outcomes

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBufferedEmitter_AppendsNDJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outcomes.ndjson")

	be, err := NewBufferedEmitter(BufferedConfig{Path: path})
	if err != nil {
		t.Fatalf("NewBufferedEmitter: %v", err)
	}
	defer be.Close()

	o1 := newValidOutcome()
	o1.Detail = "first"
	o2 := newValidOutcome()
	o2.Detail = "second"

	if err := be.Emit(context.Background(), o1); err != nil {
		t.Fatalf("emit 1: %v", err)
	}
	if err := be.Emit(context.Background(), o2); err != nil {
		t.Fatalf("emit 2: %v", err)
	}
	if err := be.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d", len(lines))
	}

	var got1, got2 Outcome
	if err := json.Unmarshal([]byte(lines[0]), &got1); err != nil {
		t.Fatalf("decode line 1: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &got2); err != nil {
		t.Fatalf("decode line 2: %v", err)
	}
	if got1.Detail != "first" {
		t.Errorf("line 1 Detail=%q want %q", got1.Detail, "first")
	}
	if got2.Detail != "second" {
		t.Errorf("line 2 Detail=%q want %q", got2.Detail, "second")
	}
	if got1.Kind != KindAgentOutcome {
		t.Errorf("line 1 Kind=%q want %q", got1.Kind, KindAgentOutcome)
	}
}

func TestBufferedEmitter_RotatesAtMaxBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outcomes.ndjson")

	be, err := NewBufferedEmitter(BufferedConfig{
		Path:     path,
		MaxBytes: 200,
		MaxFiles: 3,
	})
	if err != nil {
		t.Fatalf("NewBufferedEmitter: %v", err)
	}
	defer be.Close()

	for i := 0; i < 30; i++ {
		o := newValidOutcome()
		o.Detail = strings.Repeat("x", 50)
		if err := be.Emit(context.Background(), o); err != nil {
			t.Fatalf("emit %d: %v", i, err)
		}
	}
	if err := be.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	matches, err := filepath.Glob(path + "*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) < 2 {
		t.Errorf("expected at least 2 files after rotation, got %d (%v)", len(matches), matches)
	}
	if len(matches) > 4 {
		t.Errorf("rotation kept too many files: %d (%v)", len(matches), matches)
	}
}

func TestBufferedEmitter_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outcomes.ndjson")

	be, err := NewBufferedEmitter(BufferedConfig{Path: path})
	if err != nil {
		t.Fatalf("NewBufferedEmitter: %v", err)
	}
	defer be.Close()

	var wg sync.WaitGroup
	const writers = 5
	const eventsPerWriter = 20
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < eventsPerWriter; i++ {
				o := newValidOutcome()
				o.SessionID = "writer-" + string(rune('A'+id))
				if err := be.Emit(context.Background(), o); err != nil {
					t.Errorf("writer %d emit: %v", id, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	if err := be.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	lines := readLines(t, path)
	want := writers * eventsPerWriter
	if len(lines) != want {
		t.Errorf("expected %d lines, got %d", want, len(lines))
	}
	for i, ln := range lines {
		var o Outcome
		if err := json.Unmarshal([]byte(ln), &o); err != nil {
			t.Errorf("line %d invalid JSON: %v", i, err)
		}
	}
}

func TestBufferedEmitter_RejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outcomes.ndjson")

	be, err := NewBufferedEmitter(BufferedConfig{Path: path})
	if err != nil {
		t.Fatalf("NewBufferedEmitter: %v", err)
	}
	defer be.Close()

	if err := be.Emit(context.Background(), Outcome{Kind: KindAgentOutcome}); err == nil {
		t.Errorf("expected validation error")
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("file should NOT have been created for invalid outcome")
	}
}

func TestBufferedEmitter_NormalizesTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outcomes.ndjson")

	be, err := NewBufferedEmitter(BufferedConfig{Path: path})
	if err != nil {
		t.Fatalf("NewBufferedEmitter: %v", err)
	}
	defer be.Close()

	before := time.Now().Add(-time.Second).UTC()
	o := newValidOutcome()
	o.Timestamp = time.Time{}
	if err := be.Emit(context.Background(), o); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := be.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	lines := readLines(t, path)
	var got Outcome
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Timestamp.Before(before) {
		t.Errorf("timestamp not auto-filled: got %v", got.Timestamp)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		ln := strings.TrimSpace(scanner.Text())
		if ln == "" {
			continue
		}
		lines = append(lines, ln)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return lines
}
