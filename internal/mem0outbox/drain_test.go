package mem0outbox

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

// mockDualPusher records pushes and supports idempotency checks.
type mockDualPusher struct {
	mu    sync.Mutex
	seen  map[string]int // capsule ID -> push count
	calls int
	err   error
}

func newMockPusher() *mockDualPusher {
	return &mockDualPusher{seen: make(map[string]int)}
}

func (m *mockDualPusher) Push(ctx context.Context, c Capsule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.seen[c.ID]++
	m.calls++
	return nil
}

func TestOutbox_DrainsExactlyOnceAcrossQuotaWindow(t *testing.T) {
	dir := t.TempDir()
	pendingPath := filepath.Join(dir, "pending.jsonl")

	w, err := NewWriter(pendingPath)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 5; i++ {
		_ = w.Append(Capsule{
			ID:     "cap-" + strconv.Itoa(i),
			AppID:  "cursor-global-kb",
			UserID: "test",
			Text:   "capsule " + strconv.Itoa(i),
		})
	}

	managed := newMockPusher()
	oss := newMockPusher()

	drainer := &QuotaDrainer{
		PendingPath: pendingPath,
		CursorPath:  filepath.Join(dir, "drain-cursor"),
		Managed:     managed,
		OSS:         oss,
		BatchSize:   10,
		DryRun:      false,
	}

	report, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if report.Drained != 5 {
		t.Errorf("drained: got %d, want 5", report.Drained)
	}

	// Each capsule must be pushed exactly once to EACH backend.
	for i := 0; i < 5; i++ {
		id := "cap-" + strconv.Itoa(i)
		if managed.seen[id] != 1 {
			t.Errorf("managed push for %s: got %d, want 1", id, managed.seen[id])
		}
		if oss.seen[id] != 1 {
			t.Errorf("oss push for %s: got %d, want 1", id, oss.seen[id])
		}
	}

	// Second drain (retry) must be idempotent: zero new pushes.
	report2, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("second Drain: %v", err)
	}
	if report2.Drained != 0 {
		t.Errorf("second drain: got %d, want 0 (idempotent)", report2.Drained)
	}
}

func TestOutbox_DryRunDoesNotPush(t *testing.T) {
	dir := t.TempDir()
	pendingPath := filepath.Join(dir, "pending.jsonl")

	w, err := NewWriter(pendingPath)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 3; i++ {
		_ = w.Append(Capsule{
			ID:   "dry-" + strconv.Itoa(i),
			Text: "dry run capsule",
		})
	}

	managed := newMockPusher()
	oss := newMockPusher()

	drainer := &QuotaDrainer{
		PendingPath: pendingPath,
		CursorPath:  filepath.Join(dir, "drain-cursor"),
		Managed:     managed,
		OSS:         oss,
		BatchSize:   10,
		DryRun:      true,
	}

	report, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if report.Drained != 0 {
		t.Errorf("dry-run drained: got %d, want 0", report.Drained)
	}
	if report.Pending != 3 {
		t.Errorf("dry-run pending: got %d, want 3", report.Pending)
	}
	if managed.calls != 0 {
		t.Errorf("dry-run managed calls: got %d, want 0", managed.calls)
	}
	if oss.calls != 0 {
		t.Errorf("dry-run oss calls: got %d, want 0", oss.calls)
	}
}

func TestOutbox_DrainResumesFromCursor(t *testing.T) {
	dir := t.TempDir()
	pendingPath := filepath.Join(dir, "pending.jsonl")

	w, err := NewWriter(pendingPath)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 4; i++ {
		_ = w.Append(Capsule{
			ID:   "resume-" + strconv.Itoa(i),
			Text: "capsule",
		})
	}

	managed := newMockPusher()
	oss := newMockPusher()

	drainer := &QuotaDrainer{
		PendingPath: pendingPath,
		CursorPath:  filepath.Join(dir, "drain-cursor"),
		Managed:     managed,
		OSS:         oss,
		BatchSize:   2,
		DryRun:      false,
	}

	// First drain: batch of 2.
	report1, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain 1: %v", err)
	}
	if report1.Drained != 2 {
		t.Errorf("drain 1: got %d, want 2", report1.Drained)
	}

	// Second drain picks up where we left off.
	report2, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain 2: %v", err)
	}
	if report2.Drained != 2 {
		t.Errorf("drain 2: got %d, want 2", report2.Drained)
	}

	// Verify all 4 pushed exactly once to each backend.
	if managed.calls != 4 {
		t.Errorf("total managed: got %d, want 4", managed.calls)
	}
	if oss.calls != 4 {
		t.Errorf("total oss: got %d, want 4", oss.calls)
	}
}

func TestOutbox_DrainMissingPendingIsNoOp(t *testing.T) {
	dir := t.TempDir()
	drainer := &QuotaDrainer{
		PendingPath: filepath.Join(dir, "missing.jsonl"),
		CursorPath:  filepath.Join(dir, "cursor"),
		Managed:     newMockPusher(),
		OSS:         newMockPusher(),
	}

	report, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("missing pending: %v", err)
	}
	if report.Drained != 0 {
		t.Errorf("want 0, got %d", report.Drained)
	}
}

func TestOutbox_DrainSkipsCorruptLines(t *testing.T) {
	dir := t.TempDir()
	pendingPath := filepath.Join(dir, "pending.jsonl")
	if err := os.WriteFile(pendingPath, []byte("{broken\n{\"id\":\"ok\",\"text\":\"good\"}\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	managed := newMockPusher()
	oss := newMockPusher()

	drainer := &QuotaDrainer{
		PendingPath: pendingPath,
		CursorPath:  filepath.Join(dir, "cursor"),
		Managed:     managed,
		OSS:         oss,
		BatchSize:   10,
	}

	report, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if report.Drained != 1 {
		t.Errorf("drained: got %d, want 1", report.Drained)
	}
	if report.Skipped != 1 {
		t.Errorf("skipped: got %d, want 1", report.Skipped)
	}
}
