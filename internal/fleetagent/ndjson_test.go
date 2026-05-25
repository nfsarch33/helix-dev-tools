package fleetagent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNDJSONReporter_Report(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fleet-exec.ndjson")

	reporter, err := NewNDJSONReporter(path, "agent-1")
	if err != nil {
		t.Fatalf("failed to create reporter: %v", err)
	}

	result := ExecutionResult{
		TicketID:  "T-001",
		Success:   true,
		Output:    "all tests passed",
		Duration:  3 * time.Second,
		Timestamp: time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC),
	}

	err = reporter.Report(context.Background(), result)
	if err != nil {
		t.Fatalf("report failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var entry NDJSONEntry
	if err := json.Unmarshal(data[:len(data)-1], &entry); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if entry.AgentID != "agent-1" {
		t.Errorf("expected agent_id agent-1, got %s", entry.AgentID)
	}
	if entry.TicketID != "T-001" {
		t.Errorf("expected ticket_id T-001, got %s", entry.TicketID)
	}
	if !entry.Success {
		t.Error("expected success=true")
	}
	if entry.DurationMS != 3000 {
		t.Errorf("expected 3000ms, got %d", entry.DurationMS)
	}
}

func TestNDJSONReporter_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fleet-exec.ndjson")

	reporter, err := NewNDJSONReporter(path, "agent-1")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		err = reporter.Report(context.Background(), ExecutionResult{
			TicketID:  "T-00" + string(rune('1'+i)),
			Success:   true,
			Duration:  time.Second,
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("report %d failed: %v", i, err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("expected 3 lines, got %d", lines)
	}
}

func TestNDJSONReporter_FailedResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fleet-exec.ndjson")

	reporter, err := NewNDJSONReporter(path, "agent-1")
	if err != nil {
		t.Fatal(err)
	}

	err = reporter.Report(context.Background(), ExecutionResult{
		TicketID:  "T-ERR",
		Success:   false,
		Error:     "compilation failed",
		Duration:  500 * time.Millisecond,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	var entry NDJSONEntry
	json.Unmarshal(data[:len(data)-1], &entry)

	if entry.Success {
		t.Error("expected success=false")
	}
	if entry.Error != "compilation failed" {
		t.Errorf("expected error 'compilation failed', got %q", entry.Error)
	}
}

func TestMultiReporter_FansOut(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "r1.ndjson")
	p2 := filepath.Join(dir, "r2.ndjson")

	r1, _ := NewNDJSONReporter(p1, "a1")
	r2, _ := NewNDJSONReporter(p2, "a1")
	multi := NewMultiReporter(r1, r2)

	err := multi.Report(context.Background(), ExecutionResult{
		TicketID:  "T-M",
		Success:   true,
		Duration:  time.Second,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{p1, p2} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing file %s: %v", path, err)
		}
		if len(data) == 0 {
			t.Errorf("file %s should not be empty", path)
		}
	}
}

func TestNewNDJSONReporter_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "logs")
	path := filepath.Join(dir, "fleet.ndjson")

	_, err := NewNDJSONReporter(path, "agent-1")
	if err != nil {
		t.Fatalf("should create nested dirs: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("should be a directory")
	}
}
