package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFleetReport_Registered(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "fleet-report" {
			return
		}
	}
	t.Fatal("fleet-report command not registered on rootCmd")
}

func TestCollectAgentraceSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agentrace-mcp.ndjson")

	now := time.Now()
	events := []map[string]interface{}{
		{"ts": now.Add(-1 * time.Hour).Format(time.RFC3339), "tool": "search", "agent_id": "cursor-parent", "success": true},
		{"ts": now.Add(-2 * time.Hour).Format(time.RFC3339), "tool": "read", "agent_id": "codex", "success": false},
		{"ts": now.Add(-30 * time.Hour).Format(time.RFC3339), "tool": "old-tool", "agent_id": "old-agent", "success": true},
	}
	var buf bytes.Buffer
	for _, e := range events {
		b, _ := json.Marshal(e)
		buf.Write(b)
		buf.WriteByte('\n')
	}
	os.WriteFile(path, buf.Bytes(), 0644)

	cutoff := now.Add(-24 * time.Hour)
	sect := collectAgentraceSection(path, cutoff)

	if !sect.Available {
		t.Fatal("section should be available")
	}
	if sect.TotalEvents != 2 {
		t.Fatalf("total events = %d, want 2", sect.TotalEvents)
	}
	if sect.Errors != 1 {
		t.Fatalf("errors = %d, want 1", sect.Errors)
	}
	if sect.ByTool["search"] != 1 {
		t.Fatalf("search count = %d, want 1", sect.ByTool["search"])
	}
	if sect.ByAgent["cursor-parent"] != 1 {
		t.Fatalf("cursor-parent count = %d, want 1", sect.ByAgent["cursor-parent"])
	}
}

func TestCollectSembleSection(t *testing.T) {
	dir := t.TempDir()
	agentracePath := filepath.Join(dir, "agentrace-mcp.ndjson")
	disciplinePath := filepath.Join(dir, "semble-discipline.ndjson")

	now := time.Now()
	agentraceEvents := []map[string]interface{}{
		{"ts": now.Add(-1 * time.Hour).Format(time.RFC3339), "event": "semble_search"},
		{"ts": now.Add(-2 * time.Hour).Format(time.RFC3339), "event": "semble_search"},
	}
	disciplineEvents := []map[string]interface{}{
		{"ts": now.Add(-1 * time.Hour).Format(time.RFC3339), "event": "grep_fallback", "command": "rg foo"},
	}

	writeNDJSON(t, agentracePath, agentraceEvents)
	writeNDJSON(t, disciplinePath, disciplineEvents)

	cutoff := now.Add(-24 * time.Hour)
	sect := collectSembleSection(agentracePath, disciplinePath, cutoff)

	if sect.SembleSearches != 2 {
		t.Fatalf("semble searches = %d, want 2", sect.SembleSearches)
	}
	if sect.GrepFallbacks != 1 {
		t.Fatalf("grep fallbacks = %d, want 1", sect.GrepFallbacks)
	}
	if sect.Total != 3 {
		t.Fatalf("total = %d, want 3", sect.Total)
	}
	wantCoverage := 66.6
	if sect.CoveragePct < wantCoverage-1 || sect.CoveragePct > wantCoverage+1 {
		t.Fatalf("coverage = %.1f%%, want ~%.1f%%", sect.CoveragePct, wantCoverage)
	}
	if sect.Status != "YELLOW" {
		t.Fatalf("status = %q, want YELLOW", sect.Status)
	}
}

func TestFormatFleetReport(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	var buf bytes.Buffer

	sprint := fleetSprintSection{
		SprintID:    "v7100",
		Total:       10,
		Done:        7,
		InProgress:  2,
		Blocked:     1,
		ByStatus:    map[string]int{"done": 7, "in_progress": 2, "blocked": 1},
		ProgressPct: 70.0,
		Available:   true,
	}
	agentrace := fleetAgentraceSection{
		TotalEvents: 100,
		Errors:      5,
		ByTool:      map[string]int{"search": 60, "read": 40},
		ByAgent:     map[string]int{"cursor-parent": 80, "codex": 20},
		Available:   true,
	}
	semble := fleetSembleSection{
		SembleSearches: 90,
		GrepFallbacks:  10,
		Total:          100,
		CoveragePct:    90.0,
		Status:         "GREEN",
		Available:      true,
	}

	formatFleetReport(&buf, now, 24, sprint, agentrace, semble)
	output := buf.String()

	for _, want := range []string{
		"# Fleet Report",
		"v7100",
		"70%",
		"100",
		"GREEN",
		"Semble searches",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func writeNDJSON(t *testing.T, path string, events []map[string]interface{}) {
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
