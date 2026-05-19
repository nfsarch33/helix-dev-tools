package qasession

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	s := New(Config{SprintID: "v6310", RepoPath: "/tmp/test"})
	if s == nil {
		t.Fatal("expected non-nil session")
	}
	if s.Status() != StatusPending {
		t.Errorf("got status %v, want pending", s.Status())
	}
}

func TestAddCheck(t *testing.T) {
	s := New(Config{SprintID: "v6310"})
	s.AddCheck(Check{Name: "race-test", Command: "go test -race ./..."})
	s.AddCheck(Check{Name: "sentrux-gate", Command: "sentrux gate ."})
	checks := s.Checks()
	if len(checks) != 2 {
		t.Fatalf("got %d checks, want 2", len(checks))
	}
}

func TestCheckResult(t *testing.T) {
	s := New(Config{SprintID: "v6310"})
	s.AddCheck(Check{Name: "lint", Command: "golangci-lint run"})
	s.RecordResult("lint", CheckResult{Passed: true, Output: "no issues"})

	results := s.Results()
	if !results["lint"].Passed {
		t.Error("expected lint to pass")
	}
}

func TestAllPassed(t *testing.T) {
	s := New(Config{SprintID: "v6310"})
	s.AddCheck(Check{Name: "test", Command: "go test ./..."})
	s.AddCheck(Check{Name: "vet", Command: "go vet ./..."})
	s.RecordResult("test", CheckResult{Passed: true})
	s.RecordResult("vet", CheckResult{Passed: true})

	if !s.AllPassed() {
		t.Error("expected all passed")
	}
}

func TestAllPassedWithFailure(t *testing.T) {
	s := New(Config{SprintID: "v6310"})
	s.AddCheck(Check{Name: "test", Command: "go test ./..."})
	s.AddCheck(Check{Name: "vet", Command: "go vet ./..."})
	s.RecordResult("test", CheckResult{Passed: true})
	s.RecordResult("vet", CheckResult{Passed: false, Output: "unused variable"})

	if s.AllPassed() {
		t.Error("expected not all passed")
	}
}

func TestSentruxCheck(t *testing.T) {
	s := New(Config{SprintID: "v6310", SentruxEnabled: true})
	s.SetSentruxBaseline(7013)
	s.SetSentruxResult(7075)

	if !s.SentruxPassed() {
		t.Error("expected sentrux to pass (improved)")
	}
}

func TestSentruxRegression(t *testing.T) {
	s := New(Config{SprintID: "v6310", SentruxEnabled: true})
	s.SetSentruxBaseline(7075)
	s.SetSentruxResult(7000)

	if s.SentruxPassed() {
		t.Error("expected sentrux to fail (regressed)")
	}
}
