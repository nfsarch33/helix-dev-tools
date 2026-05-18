package bootstrap

import "testing"

func TestDefaultConfig_HasSteps(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Steps) == 0 {
		t.Fatal("default config should have steps")
	}
	if len(cfg.Steps) < 10 {
		t.Errorf("got %d steps, want at least 10", len(cfg.Steps))
	}
}

func TestVerify_AllPass(t *testing.T) {
	cfg := Config{
		Steps: []Step{
			{Name: "always-pass", Check: "true"},
			{Name: "also-pass", Check: "echo ok"},
		},
	}

	result := Verify(cfg)
	if !result.AllPassed() {
		t.Error("expected all pass")
	}
	if result.Passed != 2 {
		t.Errorf("passed = %d, want 2", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0", result.Failed)
	}
}

func TestVerify_WithFailure(t *testing.T) {
	cfg := Config{
		Steps: []Step{
			{Name: "pass", Check: "true"},
			{Name: "fail", Check: "false"},
		},
	}

	result := Verify(cfg)
	if result.AllPassed() {
		t.Error("expected failure")
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
}

func TestVerify_SkippedStep(t *testing.T) {
	cfg := Config{
		Steps: []Step{
			{Name: "skip-me", Check: "false", Skip: true},
			{Name: "run-me", Check: "true"},
		},
	}

	result := Verify(cfg)
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
	if result.Passed != 1 {
		t.Errorf("passed = %d, want 1", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0", result.Failed)
	}
}

func TestVerify_EmptyCheck(t *testing.T) {
	cfg := Config{
		Steps: []Step{
			{Name: "no-check"},
		},
	}

	result := Verify(cfg)
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (empty check)", result.Skipped)
	}
}

func TestBootstrapResult_Summary(t *testing.T) {
	result := BootstrapResult{
		TotalSteps: 5,
		Passed:     4,
		Failed:     1,
		Skipped:    0,
		DurationMS: 1500,
	}

	summary := result.Summary()
	if summary == "" {
		t.Fatal("summary should not be empty")
	}
	if result.AllPassed() {
		t.Error("should not be all passed with 1 failure")
	}
}

func TestBootstrapResult_AllPassed_True(t *testing.T) {
	result := BootstrapResult{Failed: 0}
	if !result.AllPassed() {
		t.Error("should be all passed with 0 failures")
	}
}

func TestVerify_DurationTracked(t *testing.T) {
	cfg := Config{
		Steps: []Step{{Name: "quick", Check: "true"}},
	}

	result := Verify(cfg)
	if result.DurationMS < 0 {
		t.Error("duration should be non-negative")
	}
}

func TestDefaultConfig_StepNames(t *testing.T) {
	cfg := DefaultConfig()
	names := make(map[string]bool)
	for _, step := range cfg.Steps {
		if step.Name == "" {
			t.Error("step name should not be empty")
		}
		if names[step.Name] {
			t.Errorf("duplicate step name: %s", step.Name)
		}
		names[step.Name] = true
	}
}
