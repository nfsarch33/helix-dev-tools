package conflictdetect

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestClaimFile_Success(t *testing.T) {
	d := NewDetector()
	err := d.ClaimFile("agent1", "/test/file.txt")
	if err != nil {
		t.Errorf("ClaimFile failed: %v", err)
	}
}

func TestClaimFile_Conflict(t *testing.T) {
	d := NewDetector()
	err1 := d.ClaimFile("agent1", "/test/file.txt")
	if err1 != nil {
		t.Errorf("First ClaimFile failed: %v", err1)
	}

	err2 := d.ClaimFile("agent2", "/test/file.txt")
	if err2 == nil {
		t.Error("Expected conflict error when second agent claims file")
	}
}

func TestClaimFile_SameAgent(t *testing.T) {
	d := NewDetector()
	err1 := d.ClaimFile("agent1", "/test/file.txt")
	if err1 != nil {
		t.Errorf("First ClaimFile failed: %v", err1)
	}

	err2 := d.ClaimFile("agent1", "/test/file.txt")
	if err2 != nil {
		t.Errorf("Same agent re-claiming file should not fail: %v", err2)
	}
}

func TestReleaseFile_ClearsLock(t *testing.T) {
	d := NewDetector()
	err1 := d.ClaimFile("agent1", "/test/file.txt")
	if err1 != nil {
		t.Errorf("First ClaimFile failed: %v", err1)
	}

	d.ReleaseFile("agent1", "/test/file.txt")

	err2 := d.ClaimFile("agent2", "/test/file.txt")
	if err2 != nil {
		t.Errorf("File should be claimable after release: %v", err2)
	}
}

func TestActiveConflicts_None(t *testing.T) {
	d := NewDetector()
	d.ClaimFile("agent1", "/test/file1.txt")
	d.ClaimFile("agent2", "/test/file2.txt")

	conflicts := d.ActiveConflicts()
	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts, got %d", len(conflicts))
	}
}

func TestActiveConflicts_Detected(t *testing.T) {
	d := NewDetector()
	d.ClaimFile("agent1", "/test/file.txt")
	d.ClaimFile("agent2", "/test/file.txt")

	conflicts := d.ActiveConflicts()
	if len(conflicts) == 0 {
		t.Error("Expected conflicts, got none")
	}

	if conflicts[0].AgentA != "agent1" || conflicts[0].AgentB != "agent2" {
		t.Errorf("Unexpected conflict agents: %v", conflicts[0])
	}
}

func TestLogConflict_WritesNDJSON(t *testing.T) {
	d := NewDetector()
	tmpfile, err := os.CreateTemp("", "conflict_log_test_*.ndjson")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	conflict := Conflict{
		Type:       ConflictFile,
		Resource:   "/test/file.txt",
		AgentA:     "agent1",
		AgentB:     "agent2",
		DetectedAt: time.Now(),
	}

	err = d.LogConflict(conflict, tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to log conflict: %v", err)
	}

	// Read and verify the logged conflict
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var loggedConflict Conflict
	err = json.Unmarshal(content, &loggedConflict)
	if err != nil {
		t.Fatalf("Failed to parse logged conflict: %v", err)
	}

	if loggedConflict.Resource != "/test/file.txt" || loggedConflict.AgentA != "agent1" || loggedConflict.AgentB != "agent2" {
		t.Errorf("Logged conflict does not match expected: %v", loggedConflict)
	}
}