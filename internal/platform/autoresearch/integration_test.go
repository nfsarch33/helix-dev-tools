package autoresearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFullLoopEndToEnd(t *testing.T) {
	dir := t.TempDir()

	entries := []map[string]interface{}{
		{"phase": "evaluate", "error": "context deadline exceeded", "note": "timeout in phase"},
		{"phase": "evaluate", "error": "context deadline exceeded", "note": ""},
		{"phase": "propose", "error": "connection refused", "note": ""},
		{"phase": "decide", "decision": "discard", "note": "rejected hypothesis"},
		{"phase": "evaluate", "note": "failure detected in test suite"},
	}
	agtracePath := writeTestAgentrace(t, dir, entries)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/memories/" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"mem-e2e-1"}`))
		case r.URL.Path == "/v1/memories/search/" && r.Method == http.MethodPost:
			w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	engram := &EngramClient{
		BaseURL: srv.URL,
		UserID:  "e2e-test",
		HTTP:    srv.Client(),
	}

	promotionDir := filepath.Join(dir, "promotions")
	evospinePath := filepath.Join(dir, "sentrux.ndjson")

	pcfg := ProbeConfig{
		AgentracePaths: []string{agtracePath},
		MaxEntries:     100,
	}
	promoteCfg := PromoteConfig{
		Engram:      engram,
		OutputDir:   promotionDir,
		EvoSpineLog: evospinePath,
	}

	cfg := DefaultConfig()
	cfg.MaxIterations = 1
	cfg.LogPath = filepath.Join(dir, "ar-e2e.ndjson")

	runner := New(cfg,
		NewProbePhase(pcfg),
		NewProposePhase(),
		NewEvaluatePhase(nil),
		NewDecidePhase(DefaultDecideThreshold),
		NewPromotePhase(promoteCfg),
	)

	history, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 iteration, got %d", len(history))
	}

	state := history[0]
	t.Logf("Decision: %s", state.Decision)
	t.Logf("Metric: %.4f", state.Metric)
	t.Logf("BaseMetric: %.4f", state.BaseMetric)
	t.Logf("Delta: %.4f", state.Delta)
	t.Logf("Note: %s", state.Note)

	if _, err := os.Stat(cfg.LogPath); err != nil {
		t.Errorf("agentrace log not created: %v", err)
	}
}

func TestFullLoopWithKeepPromotion(t *testing.T) {
	dir := t.TempDir()

	entries := []map[string]interface{}{
		{"phase": "evaluate", "error": "OOM killed", "note": "out of memory"},
		{"phase": "evaluate", "error": "OOM killed", "note": ""},
		{"phase": "evaluate", "error": "OOM killed", "note": ""},
	}
	agtracePath := writeTestAgentrace(t, dir, entries)

	promotionDir := filepath.Join(dir, "promotions")
	evospinePath := filepath.Join(dir, "sentrux.ndjson")

	pcfg := ProbeConfig{
		AgentracePaths: []string{agtracePath},
		MaxEntries:     100,
	}
	promoteCfg := PromoteConfig{
		OutputDir:   promotionDir,
		EvoSpineLog: evospinePath,
	}

	alwaysKeep := func(_ context.Context, s LoopState) (LoopState, error) {
		s.Decision = DecisionKeep
		s.Delta = 0.5
		s.Metric = 0.8
		s.BaseMetric = 0.3
		return s, nil
	}

	cfg := DefaultConfig()
	cfg.MaxIterations = 2
	cfg.LogPath = filepath.Join(dir, "ar-keep.ndjson")

	runner := New(cfg,
		NewProbePhase(pcfg),
		NewProposePhase(),
		NewEvaluatePhase(nil),
		alwaysKeep,
		NewPromotePhase(promoteCfg),
	)

	history, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 iterations (keep continues), got %d", len(history))
	}

	matches, _ := filepath.Glob(filepath.Join(promotionDir, "research-*.json"))
	if len(matches) != 2 {
		t.Errorf("expected 2 promotion files, got %d", len(matches))
	}

	if _, err := os.Stat(evospinePath); err != nil {
		t.Errorf("evospine log not created: %v", err)
	}
}

func TestFullLoopContextCancellation(t *testing.T) {
	dir := t.TempDir()
	entries := []map[string]interface{}{
		{"phase": "probe", "error": "test error"},
	}
	agtracePath := writeTestAgentrace(t, dir, entries)

	slowEvaluator := func(ctx context.Context, h Hypothesis) (EvalResult, error) {
		select {
		case <-ctx.Done():
			return EvalResult{}, ctx.Err()
		case <-time.After(5 * time.Second):
			return EvalResult{}, nil
		}
	}

	pcfg := ProbeConfig{AgentracePaths: []string{agtracePath}, MaxEntries: 100}
	cfg := DefaultConfig()
	cfg.MaxIterations = 1
	cfg.EvaluateBudget = 10 * time.Millisecond
	cfg.LogPath = filepath.Join(dir, "ar-ctx.ndjson")

	runner := New(cfg,
		NewProbePhase(pcfg),
		NewProposePhase(),
		NewEvaluatePhase(slowEvaluator),
		NewDecidePhase(0),
		NewPromotePhase(PromoteConfig{}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := runner.Run(ctx)
	t.Logf("run result: err=%v (expected context error or clean finish)", err)
}

func TestRunnerStatusFromHistory(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "ar-status.ndjson")

	cfg := DefaultConfig()
	cfg.MaxIterations = 3
	cfg.LogPath = logPath

	keepDecider := func(_ context.Context, s LoopState) (LoopState, error) {
		s.Decision = DecisionKeep
		s.Metric = 0.7
		s.BaseMetric = 0.4
		s.Delta = 0.3
		return s, nil
	}

	runner := New(cfg, nil, nil, nil, keepDecider, nil)
	history, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	status := BuildStatus(history, logPath)
	if status.TotalIterations != 3 {
		t.Errorf("TotalIterations: got %d, want 3", status.TotalIterations)
	}
	if status.KeepCount != 3 {
		t.Errorf("KeepCount: got %d, want 3", status.KeepCount)
	}
	if status.DiscardCount != 0 {
		t.Errorf("DiscardCount: got %d, want 0", status.DiscardCount)
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	t.Logf("Status:\n%s", data)
}
