package tailer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// waitFor polls fn every 10ms until it returns true or deadline
// elapses. It returns true on success and false on timeout.
func waitFor(deadline time.Duration, fn func() bool) bool {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if fn() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fn()
}

// startTailer launches a tailer in a goroutine and returns a cancel
// func plus a join channel.
func startTailer(t *testing.T, opts Options) (cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	tlr, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := make(chan error, 1)
	go func() { d <- tlr.Run(ctx) }()
	return cancel, d
}

func collectLines() (handler Handler, lines func() []string) {
	var (
		mu  sync.Mutex
		buf []string
	)
	handler = func(_ context.Context, line []byte) error {
		mu.Lock()
		buf = append(buf, string(line))
		mu.Unlock()
		return nil
	}
	lines = func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, len(buf))
		copy(out, buf)
		return out
	}
	return
}

// TestTailer_DetectsAppendedLine writes the seed line, starts the
// tailer, appends another line, and verifies both lines reach the
// handler.
func TestTailer_DetectsAppendedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte("line1\n"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	handler, lines := collectLines()
	cancel, done := startTailer(t, Options{
		Path:     path,
		Interval: 25 * time.Millisecond,
		Handler:  handler,
	})

	if !waitFor(2*time.Second, func() bool { return len(lines()) >= 1 }) {
		cancel()
		<-done
		t.Fatalf("did not detect seed line within 2s, got=%v", lines())
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	if _, err := f.WriteString("line2\n"); err != nil {
		t.Fatalf("append line2: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if !waitFor(2*time.Second, func() bool { return len(lines()) >= 2 }) {
		cancel()
		<-done
		t.Fatalf("did not detect line2 within 2s, got=%v", lines())
	}

	cancel()
	<-done

	got := lines()
	if len(got) != 2 {
		t.Fatalf("len(lines) = %d, want 2: %v", len(got), got)
	}
	if !strings.Contains(got[0], "line1") {
		t.Fatalf("first line = %q, want to contain 'line1'", got[0])
	}
	if !strings.Contains(got[1], "line2") {
		t.Fatalf("second line = %q, want to contain 'line2'", got[1])
	}
}

// TestTailer_HandlesFileTruncation simulates JSONL rotation: the file
// shrinks below the previous offset and is repopulated. The tailer
// must reset its offset and emit the new lines.
func TestTailer_HandlesFileTruncation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"n":1}`+"\n"+`{"n":2}`+"\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	handler, lines := collectLines()
	cancel, done := startTailer(t, Options{
		Path:     path,
		Interval: 25 * time.Millisecond,
		Handler:  handler,
	})
	defer func() {
		cancel()
		<-done
	}()

	if !waitFor(2*time.Second, func() bool { return len(lines()) >= 2 }) {
		t.Fatalf("did not consume seed lines, got=%v", lines())
	}

	if err := os.WriteFile(path, []byte(`{"n":3}`+"\n"), 0o600); err != nil {
		t.Fatalf("truncate+rewrite: %v", err)
	}

	if !waitFor(2*time.Second, func() bool { return len(lines()) >= 3 }) {
		t.Fatalf("did not detect post-truncation line, got=%v", lines())
	}

	got := lines()
	if !strings.Contains(got[len(got)-1], `"n":3`) {
		t.Fatalf("last line = %q, want post-truncation entry", got[len(got)-1])
	}
}

// TestTailer_PollsAtConfiguredInterval verifies the configured Interval
// is honoured: with a 50ms interval the tailer must detect a freshly
// appended line within one full poll cycle (250ms safety budget).
func TestTailer_PollsAtConfiguredInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	handler, lines := collectLines()
	interval := 50 * time.Millisecond
	cancel, done := startTailer(t, Options{
		Path:     path,
		Interval: interval,
		Handler:  handler,
	})
	defer func() {
		cancel()
		<-done
	}()

	// Wait long enough for the tailer to have completed its initial
	// scan of the empty file and entered the poll loop.
	time.Sleep(2 * interval)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	start := time.Now()
	if _, err := f.WriteString("hello\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if !waitFor(5*interval, func() bool { return len(lines()) >= 1 }) {
		t.Fatalf("did not detect appended line within 5x interval (%v)", 5*interval)
	}
	elapsed := time.Since(start)
	if elapsed > 5*interval {
		t.Fatalf("detection latency %v exceeds 5x interval %v", elapsed, 5*interval)
	}
}

// TestNew_RejectsInvalidConfig pins the input contract: nil handler,
// empty path, and non-positive interval are all configuration errors.
func TestNew_RejectsInvalidConfig(t *testing.T) {
	noopHandler := func(_ context.Context, _ []byte) error { return nil }

	cases := []struct {
		name string
		opts Options
	}{
		{name: "no handler", opts: Options{Path: "/tmp/x", Interval: time.Second}},
		{name: "no path", opts: Options{Handler: noopHandler, Interval: time.Second}},
		{name: "zero interval", opts: Options{Path: "/tmp/x", Handler: noopHandler}},
		{name: "negative interval", opts: Options{Path: "/tmp/x", Handler: noopHandler, Interval: -time.Second}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.opts); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}
