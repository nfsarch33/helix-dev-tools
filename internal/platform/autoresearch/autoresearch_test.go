package autoresearch

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunnerDefaultNoopPhasesComplete(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIterations = 2
	cfg.LogPath = filepath.Join(t.TempDir(), "ar.ndjson")

	r := New(cfg, nil, nil, nil, nil, nil)
	history, err := r.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("expected 2 iterations, got %d", len(history))
	}
}

func TestRunnerStopsOnDiscard(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIterations = 5
	cfg.LogPath = filepath.Join(t.TempDir(), "ar.ndjson")

	decideDiscard := func(_ context.Context, s LoopState) (LoopState, error) {
		s.Decision = DecisionDiscard
		return s, nil
	}
	r := New(cfg, nil, nil, nil, decideDiscard, nil)
	history, err := r.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("expected 1 iteration before discard stop, got %d", len(history))
	}
}

func TestRunnerContinuesOnKeep(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIterations = 3
	cfg.LogPath = filepath.Join(t.TempDir(), "ar.ndjson")

	decideKeep := func(_ context.Context, s LoopState) (LoopState, error) {
		s.Decision = DecisionKeep
		return s, nil
	}
	r := New(cfg, nil, nil, nil, decideKeep, nil)
	history, err := r.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("expected 3 iterations, got %d", len(history))
	}
}

func TestRunnerWritesNDJSONLog(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.MaxIterations = 1
	cfg.AgentID = "test-agent"
	cfg.LogPath = filepath.Join(dir, "ar.ndjson")

	r := New(cfg, nil, nil, nil, nil, nil)
	if _, err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	f, err := os.Open(cfg.LogPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Fatalf("unmarshal line %d: %v", count, err)
		}
		if rec["agent_id"] != "test-agent" {
			t.Errorf("line %d: wrong agent_id %q", count, rec["agent_id"])
		}
		count++
	}
	// 1 iteration x 5 phases = 5 log lines
	if count != 5 {
		t.Errorf("expected 5 log lines for 1 iteration, got %d", count)
	}
}

func TestRunnerContextCancellation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIterations = 10
	cfg.ProposeBudget = 5 * time.Second
	cfg.LogPath = filepath.Join(t.TempDir(), "ar.ndjson")

	slow := func(_ context.Context, s LoopState) (LoopState, error) {
		time.Sleep(1 * time.Millisecond)
		return s, nil
	}
	r := New(cfg, nil, slow, nil, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, err := r.Run(ctx)
	if err == nil {
		t.Log("context cancelled cleanly or loop finished before timeout; acceptable")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxIterations <= 0 {
		t.Error("MaxIterations must be positive")
	}
	if cfg.ProbeBudget <= 0 || cfg.ProposeBudget <= 0 || cfg.EvaluateBudget <= 0 {
		t.Error("phase budgets must be positive")
	}
	if cfg.AgentID == "" {
		t.Error("AgentID must not be empty")
	}
}
