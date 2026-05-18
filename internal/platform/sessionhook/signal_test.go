package sessionhook

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordSignalWritesNDJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "signals.ndjson")

	err := RecordSignal("claude-code", SignalAccept, "internal/foo/foo.go", "v6184", "T-001", "", logPath)
	if err != nil {
		t.Fatalf("RecordSignal: %v", err)
	}

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	var ev SignalEvent
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("expected one NDJSON line")
	}
	if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if ev.AgentID != "claude-code" {
		t.Errorf("agent_id: got %q, want claude-code", ev.AgentID)
	}
	if ev.Signal != SignalAccept {
		t.Errorf("signal: got %q, want accept", ev.Signal)
	}
	if ev.FilePath != "internal/foo/foo.go" {
		t.Errorf("file_path: got %q", ev.FilePath)
	}
	if ev.SprintID != "v6184" {
		t.Errorf("sprint_id: got %q", ev.SprintID)
	}
	if ev.TicketID != "T-001" {
		t.Errorf("ticket_id: got %q", ev.TicketID)
	}
	if ev.Timestamp == "" {
		t.Error("timestamp must not be empty")
	}
}

func TestRecordSignalAllTypes(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "signals.ndjson")

	signals := []SignalType{SignalAccept, SignalReject, SignalEdit, SignalRevert}
	for _, sig := range signals {
		if err := RecordSignal("claude-code", sig, "file.go", "", "", "", logPath); err != nil {
			t.Fatalf("RecordSignal(%s): %v", sig, err)
		}
	}

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var ev SignalEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			t.Fatalf("line %d unmarshal: %v", count, err)
		}
		if ev.Signal != signals[count] {
			t.Errorf("line %d: got %q, want %q", count, ev.Signal, signals[count])
		}
		count++
	}
	if count != 4 {
		t.Errorf("expected 4 lines, got %d", count)
	}
}

func TestRecordSignalCreatesDir(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nested", "deep", "signals.ndjson")

	if err := RecordSignal("claude-code", SignalEdit, "", "", "", "test", logPath); err != nil {
		t.Fatalf("RecordSignal with nested dir: %v", err)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log file not created: %v", err)
	}
}

func TestRecordSignalAppendsMultiple(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "signals.ndjson")

	for i := 0; i < 3; i++ {
		if err := RecordSignal("agent", SignalAccept, "f.go", "", "", "", logPath); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if count != 3 {
		t.Errorf("expected 3 appended lines, got %d", count)
	}
}
