package agentbench

import (
	"testing"
	"time"
)

func TestNewBenchmark(t *testing.T) {
	b := New(Config{Name: "overnight-v6310", AgentID: "cursor-parent"})
	if b == nil {
		t.Fatal("expected non-nil benchmark")
	}
}

func TestRecordRun(t *testing.T) {
	b := New(Config{Name: "test", AgentID: "test"})
	b.RecordRun(Run{
		SessionID:   "session-1",
		PackageCount: 5,
		TestCount:   30,
		Duration:    25 * time.Minute,
		PassRate:    1.0,
	})
	runs := b.Runs()
	if len(runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(runs))
	}
}

func TestCompare(t *testing.T) {
	b := New(Config{Name: "test", AgentID: "test"})
	b.RecordRun(Run{SessionID: "s1", PackageCount: 3, TestCount: 15, Duration: 30 * time.Minute, PassRate: 0.8})
	b.RecordRun(Run{SessionID: "s2", PackageCount: 5, TestCount: 30, Duration: 25 * time.Minute, PassRate: 1.0})

	cmp := b.Compare()
	if cmp.VelocityDelta <= 0 {
		t.Errorf("expected positive velocity delta, got %f", cmp.VelocityDelta)
	}
	if cmp.PassRateDelta < 0.19 || cmp.PassRateDelta > 0.21 {
		t.Errorf("got pass rate delta %f, want ~0.2", cmp.PassRateDelta)
	}
}

func TestBestRun(t *testing.T) {
	b := New(Config{Name: "test", AgentID: "test"})
	b.RecordRun(Run{SessionID: "s1", PackageCount: 3, TestCount: 15, Duration: 15 * time.Minute, PassRate: 0.8})
	b.RecordRun(Run{SessionID: "s2", PackageCount: 5, TestCount: 30, Duration: 25 * time.Minute, PassRate: 1.0})

	best := b.BestRun()
	if best.SessionID != "s2" {
		t.Errorf("got best %q, want s2", best.SessionID)
	}
}

func TestVelocityTrend(t *testing.T) {
	b := New(Config{Name: "test", AgentID: "test"})
	b.RecordRun(Run{SessionID: "s1", PackageCount: 3, Duration: 15 * time.Minute})
	b.RecordRun(Run{SessionID: "s2", PackageCount: 5, Duration: 25 * time.Minute})
	b.RecordRun(Run{SessionID: "s3", PackageCount: 8, Duration: 40 * time.Minute})

	trend := b.VelocityTrend()
	if len(trend) != 3 {
		t.Fatalf("got %d trend points, want 3", len(trend))
	}
}
