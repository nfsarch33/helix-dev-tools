package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withCapturedStdout captures os.Stdout during fn() and returns what was
// written. The mem0-outbox dispatcher prints results via fmt.Print* so
// asserting on the returned string is the cleanest way to verify the
// branches that do not call the live Mem0 client.
func withCapturedStdout(t *testing.T, fn func()) string {
	t.Helper()
	saved := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = saved
	}()
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		defer close(done)
		_, _ = io.Copy(&buf, r)
	}()
	fn()
	_ = w.Close()
	<-done
	_ = r.Close()
	return buf.String()
}

func resetMem0OutboxFlags(t *testing.T, root string) {
	t.Helper()
	mem0OutboxFlags.root = root
	mem0OutboxFlags.apiKey = ""
	mem0OutboxFlags.baseURL = "https://api.mem0.ai"
	mem0OutboxFlags.mcpJSON = ""
	mem0OutboxFlags.batchSize = 50
	mem0OutboxFlags.maxIterations = 0
	mem0OutboxFlags.flushIntervalS = 30
	mem0OutboxFlags.tail = 0
	mem0OutboxFlags.once = false
	mem0OutboxFlags.dryRun = false
}

func TestRunMem0Outbox_TailEmptyDirReturnsNoCapsules(t *testing.T) {
	dir := t.TempDir()
	resetMem0OutboxFlags(t, dir)
	mem0OutboxFlags.tail = 5

	var err error
	out := withCapturedStdout(t, func() {
		err = runMem0Outbox(nil, nil)
	})
	if err != nil {
		t.Fatalf("runMem0Outbox tail: %v", err)
	}
	if !strings.Contains(out, "cursor-tools mem0-outbox tail") {
		t.Fatalf("missing tail header: %q", out)
	}
}

func TestRunMem0Outbox_TailListsRecentCapsules(t *testing.T) {
	dir := t.TempDir()
	pending := filepath.Join(dir, "pending.jsonl")
	const sample = `{"id":"abc-1","text":"hello","app_id":"cursor-global-kb","user_id":"jason"}` + "\n" +
		`{"id":"abc-2","text":"world","app_id":"cursor-global-kb","user_id":"jason"}` + "\n"
	if err := os.WriteFile(pending, []byte(sample), 0o644); err != nil {
		t.Fatalf("seed pending: %v", err)
	}
	resetMem0OutboxFlags(t, dir)
	mem0OutboxFlags.tail = 2

	var err error
	out := withCapturedStdout(t, func() {
		err = runMem0Outbox(nil, nil)
	})
	if err != nil {
		t.Fatalf("runMem0Outbox tail: %v", err)
	}
	if !strings.Contains(out, "id=abc-2") {
		t.Fatalf("expected newest id first: %q", out)
	}
	if !strings.Contains(out, "id=abc-1") {
		t.Fatalf("expected both ids in output: %q", out)
	}
	// Newest-first ordering.
	if strings.Index(out, "id=abc-2") > strings.Index(out, "id=abc-1") {
		t.Fatalf("tail output not newest-first: %q", out)
	}
}

func TestRunMem0Outbox_DryRunReportsBytesWithoutNetwork(t *testing.T) {
	dir := t.TempDir()
	pending := filepath.Join(dir, "pending.jsonl")
	const sample = `{"id":"abc-1","text":"hello"}` + "\n"
	if err := os.WriteFile(pending, []byte(sample), 0o644); err != nil {
		t.Fatalf("seed pending: %v", err)
	}
	resetMem0OutboxFlags(t, dir)
	mem0OutboxFlags.dryRun = true

	var err error
	out := withCapturedStdout(t, func() {
		err = runMem0Outbox(nil, nil)
	})
	if err != nil {
		t.Fatalf("runMem0Outbox dry-run: %v", err)
	}
	if !strings.Contains(out, "pending bytes:") {
		t.Fatalf("missing pending bytes line: %q", out)
	}
	if !strings.Contains(out, "flush would batch up to 50") {
		t.Fatalf("missing batch size line: %q", out)
	}
}

func TestRunMem0Outbox_DryRunOnEmptyDirReportsEmptyOutbox(t *testing.T) {
	dir := t.TempDir()
	resetMem0OutboxFlags(t, dir)
	mem0OutboxFlags.dryRun = true

	var err error
	out := withCapturedStdout(t, func() {
		err = runMem0Outbox(nil, nil)
	})
	if err != nil {
		t.Fatalf("runMem0Outbox dry-run: %v", err)
	}
	if !strings.Contains(out, "outbox empty") {
		t.Fatalf("expected empty-outbox message: %q", out)
	}
}

func TestAbbreviate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"short", "hello", 10, "hello"},
		{"newlines collapsed", "a\nb\nc", 10, "a b c"},
		{"truncated", "abcdefghij", 5, "abcde..."},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := abbreviate(c.in, c.max)
			if got != c.want {
				t.Fatalf("abbreviate(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
			}
		})
	}
}
