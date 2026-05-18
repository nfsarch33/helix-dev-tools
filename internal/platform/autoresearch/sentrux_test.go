package autoresearch

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSentruxPluginWritesMetric(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sentrux.ndjson")

	err := SentruxPlugin("test-agent", 1, 7500.0, 7200.0, "/tmp/repo", logPath)
	if err != nil {
		t.Fatalf("SentruxPlugin: %v", err)
	}

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("expected one line")
	}
	var m SentruxMetric
	if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.AgentID != "test-agent" {
		t.Errorf("agent_id: got %q", m.AgentID)
	}
	if m.Score != 7500.0 {
		t.Errorf("score: got %v", m.Score)
	}
	if m.BaseScore != 7200.0 {
		t.Errorf("base_score: got %v", m.BaseScore)
	}
	if m.Delta != 300.0 {
		t.Errorf("delta: got %v, want 300.0", m.Delta)
	}
	if m.RepoPath != "/tmp/repo" {
		t.Errorf("repo_path: got %q", m.RepoPath)
	}
}

func TestSentruxPluginCreatesDir(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nested", "metrics.ndjson")

	if err := SentruxPlugin("a", 1, 100, 90, "", logPath); err != nil {
		t.Fatalf("SentruxPlugin: %v", err)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log not created: %v", err)
	}
}

func TestSentruxPluginAppendsMultiple(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sentrux.ndjson")

	for i := 0; i < 3; i++ {
		if err := SentruxPlugin("a", i+1, float64(100+i), 100, "", logPath); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}

	f, _ := os.Open(logPath)
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if count != 3 {
		t.Errorf("expected 3 lines, got %d", count)
	}
}
