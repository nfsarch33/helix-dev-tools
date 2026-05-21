package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
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

func TestCollectSprintSection_WithTempDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-sprint.db")

	store, err := sprintboard.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	if err := store.CreateSprint(sprintboard.Sprint{
		ID:     "v7400-test",
		Name:   "Test Sprint",
		Status: sprintboard.SprintActive,
	}); err != nil {
		t.Fatalf("create sprint: %v", err)
	}

	tickets := []struct {
		id     string
		status sprintboard.TicketStatus
	}{
		{"T-1", sprintboard.StatusDone},
		{"T-2", sprintboard.StatusDone},
		{"T-3", sprintboard.StatusInProgress},
		{"T-4", sprintboard.StatusBlocked},
		{"T-5", sprintboard.StatusBacklog},
	}
	for _, tc := range tickets {
		if err := store.CreateTicket(sprintboard.Ticket{
			ID:       tc.id,
			SprintID: "v7400-test",
			Title:    "Ticket " + tc.id,
			Status:   tc.status,
		}); err != nil {
			t.Fatalf("create ticket %s: %v", tc.id, err)
		}
	}
	store.Close()

	origFn := sprintboard.DefaultDBPath
	sprintboard.DefaultDBPath = func() string { return dbPath }
	defer func() { sprintboard.DefaultDBPath = origFn }()

	sect := collectSprintSection("v7400-test")
	if !sect.Available {
		t.Fatal("section should be available")
	}
	if sect.SprintID != "v7400-test" {
		t.Fatalf("sprint ID = %q, want v7400-test", sect.SprintID)
	}
	if sect.Total != 5 {
		t.Fatalf("total = %d, want 5", sect.Total)
	}
	if sect.Done != 2 {
		t.Fatalf("done = %d, want 2", sect.Done)
	}
	if sect.InProgress != 1 {
		t.Fatalf("in_progress = %d, want 1", sect.InProgress)
	}
	if sect.Blocked != 1 {
		t.Fatalf("blocked = %d, want 1", sect.Blocked)
	}
	wantPct := 40.0
	if sect.ProgressPct < wantPct-0.1 || sect.ProgressPct > wantPct+0.1 {
		t.Fatalf("progress = %.1f%%, want %.1f%%", sect.ProgressPct, wantPct)
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
