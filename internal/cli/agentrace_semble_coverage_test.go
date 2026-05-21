package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunSembleCoverage_EmptyLog(t *testing.T) {
	dir := t.TempDir()
	agentrace := filepath.Join(dir, "agentrace-mcp.ndjson")
	discipline := filepath.Join(dir, "semble-discipline.ndjson")
	os.WriteFile(agentrace, nil, 0644)
	os.WriteFile(discipline, nil, 0644)

	err := runSembleCoverage(agentrace, discipline, 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSembleCoverage_MixedEvents(t *testing.T) {
	dir := t.TempDir()
	agentrace := filepath.Join(dir, "agentrace-mcp.ndjson")
	discipline := filepath.Join(dir, "semble-discipline.ndjson")

	now := time.Now()
	agentraceEvents := []map[string]interface{}{
		{"ts": now.Add(-1 * time.Hour).Format(time.RFC3339), "event": "semble_search"},
		{"ts": now.Add(-2 * time.Hour).Format(time.RFC3339), "event": "semble_search"},
		{"ts": now.Add(-3 * time.Hour).Format(time.RFC3339), "event": "semble_search"},
	}
	disciplineEvents := []map[string]interface{}{
		{"ts": now.Add(-1 * time.Hour).Format(time.RFC3339), "event": "grep_fallback", "command": "rg foo ."},
		{"ts": now.Add(-2 * time.Hour).Format(time.RFC3339), "event": "semble_discipline", "command": "grep bar"},
	}

	writeCoverageNDJSON(t, agentrace, agentraceEvents)
	writeCoverageNDJSON(t, discipline, disciplineEvents)

	err := runSembleCoverage(agentrace, discipline, 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSembleCoverage_TimeFiltering(t *testing.T) {
	dir := t.TempDir()
	agentrace := filepath.Join(dir, "agentrace-mcp.ndjson")
	discipline := filepath.Join(dir, "semble-discipline.ndjson")

	now := time.Now()
	agentraceEvents := []map[string]interface{}{
		{"ts": now.Add(-1 * time.Hour).Format(time.RFC3339), "event": "semble_search"},
		{"ts": now.Add(-48 * time.Hour).Format(time.RFC3339), "event": "semble_search"},
	}
	disciplineEvents := []map[string]interface{}{
		{"ts": now.Add(-1 * time.Hour).Format(time.RFC3339), "event": "grep_fallback", "command": "rg old"},
		{"ts": now.Add(-48 * time.Hour).Format(time.RFC3339), "event": "grep_fallback", "command": "rg ancient"},
	}

	writeCoverageNDJSON(t, agentrace, agentraceEvents)
	writeCoverageNDJSON(t, discipline, disciplineEvents)

	err := runSembleCoverage(agentrace, discipline, 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSembleCoverage_MissingFiles(t *testing.T) {
	dir := t.TempDir()
	agentrace := filepath.Join(dir, "nonexistent-agentrace.ndjson")
	discipline := filepath.Join(dir, "nonexistent-discipline.ndjson")

	err := runSembleCoverage(agentrace, discipline, 24)
	if err != nil {
		t.Fatalf("missing files should not error: %v", err)
	}
}

func TestRunSembleCoverage_TopPatterns(t *testing.T) {
	dir := t.TempDir()
	agentrace := filepath.Join(dir, "agentrace-mcp.ndjson")
	discipline := filepath.Join(dir, "semble-discipline.ndjson")

	now := time.Now()
	os.WriteFile(agentrace, nil, 0644)

	var disciplineEvents []map[string]interface{}
	for i := 0; i < 10; i++ {
		disciplineEvents = append(disciplineEvents, map[string]interface{}{
			"ts": now.Add(-time.Duration(i+1) * time.Minute).Format(time.RFC3339),
			"event":   "grep_fallback",
			"command": "rg repeated-pattern .",
		})
	}
	disciplineEvents = append(disciplineEvents, map[string]interface{}{
		"ts":      now.Add(-30 * time.Minute).Format(time.RFC3339),
		"event":   "grep_fallback",
		"command": "rg unique-pattern .",
	})

	writeCoverageNDJSON(t, discipline, disciplineEvents)

	err := runSembleCoverage(agentrace, discipline, 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeCoverageNDJSON(t *testing.T, path string, events []map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	for _, e := range events {
		b, _ := json.Marshal(e)
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}
