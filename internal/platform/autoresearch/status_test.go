package autoresearch

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildStatusEmpty(t *testing.T) {
	s := BuildStatus(nil, "/nonexistent")
	if s.TotalIterations != 0 {
		t.Errorf("TotalIterations: got %d, want 0", s.TotalIterations)
	}
	if s.KeepCount != 0 {
		t.Errorf("KeepCount: got %d, want 0", s.KeepCount)
	}
}

func TestBuildStatusMixed(t *testing.T) {
	history := []LoopState{
		{Iteration: 1, Decision: DecisionKeep, Metric: 0.8, Delta: 0.3, Timestamp: time.Now()},
		{Iteration: 2, Decision: DecisionDiscard, Metric: 0.4, Delta: -0.1, Timestamp: time.Now()},
	}
	s := BuildStatus(history, "")
	if s.TotalIterations != 2 {
		t.Errorf("TotalIterations: got %d, want 2", s.TotalIterations)
	}
	if s.KeepCount != 1 {
		t.Errorf("KeepCount: got %d, want 1", s.KeepCount)
	}
	if s.DiscardCount != 1 {
		t.Errorf("DiscardCount: got %d, want 1", s.DiscardCount)
	}
	if s.LastDecision != DecisionDiscard {
		t.Errorf("LastDecision: got %q, want discard", s.LastDecision)
	}
}

func TestLoadStatusFromLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "ar.ndjson")

	cfg := DefaultConfig()
	cfg.MaxIterations = 2
	cfg.LogPath = logPath
	cfg.AgentID = "status-test"

	keep := func(_ context.Context, s LoopState) (LoopState, error) {
		s.Decision = DecisionKeep
		s.Metric = 0.75
		s.BaseMetric = 0.50
		s.Delta = 0.25
		return s, nil
	}
	r := New(cfg, nil, nil, nil, keep, nil)
	if _, err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	status, err := LoadStatusFromLog(logPath)
	if err != nil {
		t.Fatalf("LoadStatusFromLog: %v", err)
	}
	if status.TotalIterations != 2 {
		t.Errorf("TotalIterations: got %d, want 2", status.TotalIterations)
	}
	// 2 iterations x 5 phases = 10 lines, but only decide phase sets decision
	if status.KeepCount < 1 {
		t.Errorf("KeepCount: got %d, want >= 1", status.KeepCount)
	}
	if status.LogEntries != 10 {
		t.Errorf("LogEntries: got %d, want 10", status.LogEntries)
	}
}

func TestLoadStatusFromLogMissing(t *testing.T) {
	_, err := LoadStatusFromLog("/nonexistent/path.ndjson")
	if err == nil {
		t.Error("expected error for missing log file")
	}
}

func TestCountLogLines(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "ar.ndjson")

	cfg := DefaultConfig()
	cfg.MaxIterations = 1
	cfg.LogPath = logPath
	r := New(cfg, nil, nil, nil, nil, nil)
	if _, err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	count := countLogLines(logPath)
	if count != 5 {
		t.Errorf("expected 5 lines (1 iter x 5 phases), got %d", count)
	}
}

func TestCountLogLinesNonexistent(t *testing.T) {
	count := countLogLines("/nonexistent/path.ndjson")
	if count != 0 {
		t.Errorf("expected 0 for nonexistent file, got %d", count)
	}
}
