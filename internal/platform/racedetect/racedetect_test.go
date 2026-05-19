package racedetect

import (
	"fmt"
	"sync"
	"testing"
)

func TestSimulateConcurrentClaims_AtomicClaim(t *testing.T) {
	var mu sync.Mutex
	claimed := ""

	claimFn := func(ticketID, agentID string) error {
		mu.Lock()
		defer mu.Unlock()
		if claimed != "" {
			return fmt.Errorf("already claimed by %s", claimed)
		}
		claimed = agentID
		return nil
	}

	agents := []string{"cursor", "claude-code", "codex"}
	report := SimulateConcurrentClaims("T-1", agents, claimFn)

	if err := ValidateAtomicity(report); err != nil {
		t.Errorf("atomicity violated: %v", err)
	}
	if report.SuccessfulClaims != 1 {
		t.Errorf("expected 1 successful claim, got %d", report.SuccessfulClaims)
	}
	if report.RejectedClaims != 2 {
		t.Errorf("expected 2 rejected claims, got %d", report.RejectedClaims)
	}
}

func TestSimulateConcurrentClaims_RaceCondition(t *testing.T) {
	claimFn := func(ticketID, agentID string) error {
		return nil
	}

	agents := []string{"a", "b", "c"}
	report := SimulateConcurrentClaims("T-1", agents, claimFn)

	if report.DoubleClaims == 0 {
		t.Error("expected double claims when no atomicity guarantee")
	}
	if err := ValidateAtomicity(report); err == nil {
		t.Error("expected atomicity validation to fail")
	}
}

func TestDetectFileConflicts_NoConflict(t *testing.T) {
	edits := map[string][]string{
		"cursor":      {"pkg/a.go", "pkg/b.go"},
		"claude-code": {"pkg/c.go", "pkg/d.go"},
	}

	conflicts := DetectFileConflicts(nil, edits)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %v", conflicts)
	}
}

func TestDetectFileConflicts_HasConflict(t *testing.T) {
	edits := map[string][]string{
		"cursor":      {"pkg/shared.go", "pkg/a.go"},
		"claude-code": {"pkg/shared.go", "pkg/b.go"},
	}

	conflicts := DetectFileConflicts(nil, edits)
	if len(conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d: %v", len(conflicts), conflicts)
	}
}

func TestDetectFileConflicts_MultipleConflicts(t *testing.T) {
	edits := map[string][]string{
		"a": {"x.go", "y.go"},
		"b": {"x.go", "y.go", "z.go"},
		"c": {"z.go"},
	}

	conflicts := DetectFileConflicts(nil, edits)
	if len(conflicts) < 2 {
		t.Errorf("expected at least 2 conflicts, got %d", len(conflicts))
	}
}

func TestValidateAtomicity_Pass(t *testing.T) {
	report := RaceReport{SuccessfulClaims: 1, DoubleClaims: 0}
	if err := ValidateAtomicity(report); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestValidateAtomicity_Fail(t *testing.T) {
	report := RaceReport{SuccessfulClaims: 3, DoubleClaims: 2}
	if err := ValidateAtomicity(report); err == nil {
		t.Error("expected failure for double claims")
	}
}
