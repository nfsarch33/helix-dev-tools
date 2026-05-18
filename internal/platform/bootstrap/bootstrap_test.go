package bootstrap

import (
	"errors"
	"testing"
)

func TestRunner_AllPass(t *testing.T) {
	r := NewRunner()
	r.Register("step1", func() (string, error) { return "ok", nil })
	r.Register("step2", func() (string, error) { return "done", nil })
	report := r.Run()
	if !report.Passed() {
		t.Error("expected all steps to pass")
	}
	if report.PassCount() != 2 {
		t.Errorf("expected 2 passed, got %d", report.PassCount())
	}
}

func TestRunner_OneFails(t *testing.T) {
	r := NewRunner()
	r.Register("good", func() (string, error) { return "ok", nil })
	r.Register("bad", func() (string, error) { return "", errors.New("missing tool") })
	report := r.Run()
	if report.Passed() {
		t.Error("expected report to not be all-pass")
	}
	failed := report.FailedSteps()
	if len(failed) != 1 || failed[0] != "bad" {
		t.Errorf("expected [bad] in failed steps, got %v", failed)
	}
}

func TestRunner_EmptySteps(t *testing.T) {
	r := NewRunner()
	report := r.Run()
	if !report.Passed() {
		t.Error("empty runner should report passed")
	}
	if report.PassCount() != 0 {
		t.Errorf("expected 0 pass count, got %d", report.PassCount())
	}
}

func TestReport_Timestamps(t *testing.T) {
	r := NewRunner()
	r.Register("noop", func() (string, error) { return "", nil })
	report := r.Run()
	if report.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
	if report.FinishedAt.IsZero() {
		t.Error("expected non-zero FinishedAt")
	}
	if report.FinishedAt.Before(report.StartedAt) {
		t.Error("FinishedAt must not be before StartedAt")
	}
}

func TestStepResult_Duration(t *testing.T) {
	r := NewRunner()
	r.Register("quick", func() (string, error) { return "fast", nil })
	report := r.Run()
	if report.Steps[0].Duration < 0 {
		t.Error("step duration must be non-negative")
	}
}
