package fleetagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailySummary_Generate_NoFile(t *testing.T) {
	ds := &DailySummary{LogPath: "/nonexistent/fleet.ndjson", AgentID: "test"}
	stats, err := ds.Generate(time.Now())
	if err != nil {
		t.Fatalf("should handle missing file: %v", err)
	}
	if stats.TotalTasks != 0 {
		t.Errorf("expected 0 tasks, got %d", stats.TotalTasks)
	}
}

func TestDailySummary_Generate_WithEntries(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fleet.ndjson")

	lines := []string{
		`{"ts":"2026-05-26T10:00:00Z","agent_id":"a1","ticket_id":"T-001","success":true,"duration_ms":3000}`,
		`{"ts":"2026-05-26T11:00:00Z","agent_id":"a1","ticket_id":"T-002","success":true,"duration_ms":5000}`,
		`{"ts":"2026-05-26T12:00:00Z","agent_id":"a1","ticket_id":"T-003","success":false,"duration_ms":1000,"error":"build failed"}`,
	}
	os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	ds := &DailySummary{LogPath: logPath, AgentID: "a1"}
	stats, err := ds.Generate(time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.TotalTasks != 3 {
		t.Errorf("expected 3 tasks, got %d", stats.TotalTasks)
	}
	if stats.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", stats.Succeeded)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}
	if stats.TotalDurationMS != 9000 {
		t.Errorf("expected 9000ms total, got %d", stats.TotalDurationMS)
	}
	if stats.AvgDurationMS != 3000 {
		t.Errorf("expected 3000ms avg, got %d", stats.AvgDurationMS)
	}
}

func TestDailySummary_Generate_FiltersByDate(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fleet.ndjson")

	lines := []string{
		`{"ts":"2026-05-25T10:00:00Z","agent_id":"a1","ticket_id":"T-OLD","success":true,"duration_ms":1000}`,
		`{"ts":"2026-05-26T10:00:00Z","agent_id":"a1","ticket_id":"T-TODAY","success":true,"duration_ms":2000}`,
		`{"ts":"2026-05-27T10:00:00Z","agent_id":"a1","ticket_id":"T-FUTURE","success":true,"duration_ms":3000}`,
	}
	os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	ds := &DailySummary{LogPath: logPath, AgentID: "a1"}
	stats, _ := ds.Generate(time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC))

	if stats.TotalTasks != 1 {
		t.Errorf("expected 1 task for today, got %d", stats.TotalTasks)
	}
}

func TestDailySummary_Generate_FiltersByAgent(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fleet.ndjson")

	lines := []string{
		`{"ts":"2026-05-26T10:00:00Z","agent_id":"a1","ticket_id":"T-1","success":true,"duration_ms":1000}`,
		`{"ts":"2026-05-26T11:00:00Z","agent_id":"a2","ticket_id":"T-2","success":true,"duration_ms":2000}`,
	}
	os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	ds := &DailySummary{LogPath: logPath, AgentID: "a1"}
	stats, _ := ds.Generate(time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC))

	if stats.TotalTasks != 1 {
		t.Errorf("expected 1 task for agent a1, got %d", stats.TotalTasks)
	}
}

func TestSummaryStats_ToMarkdown(t *testing.T) {
	stats := SummaryStats{
		Date:            "2026-05-26",
		AgentID:         "fleet-gpu-host-1",
		TotalTasks:      10,
		Succeeded:       8,
		Failed:          2,
		SuccessRate:     0.8,
		TotalDurationMS: 30000,
		AvgDurationMS:   3000,
	}

	md := stats.ToMarkdown()
	checks := []string{
		"Fleet Agent Daily Summary",
		"2026-05-26",
		"fleet-gpu-host-1",
		"80.0%",
		"30000ms",
		"3000ms",
	}
	for _, check := range checks {
		if !strings.Contains(md, check) {
			t.Errorf("markdown should contain %q", check)
		}
	}
}

func TestDailySummary_EmptyStats(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "fleet.ndjson")
	os.WriteFile(logPath, nil, 0644)

	ds := &DailySummary{LogPath: logPath, AgentID: "a1"}
	stats, _ := ds.Generate(time.Now())

	if stats.SuccessRate != 0 {
		t.Error("success rate should be 0 for empty stats")
	}
	if stats.AvgDurationMS != 0 {
		t.Error("avg duration should be 0 for empty stats")
	}
}
