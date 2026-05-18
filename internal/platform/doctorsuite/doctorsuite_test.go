package doctorsuite

import "testing"

func TestSuite_AllPass(t *testing.T) {
	s := NewSuite()
	s.Register("go-version", func() CheckResult {
		return CheckResult{Name: "go-version", Level: LevelPass, Message: "go 1.25"}
	})
	s.Register("sentrux", func() CheckResult {
		return CheckResult{Name: "sentrux", Level: LevelPass, Message: "7058"}
	})
	results := s.Run()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Level != LevelPass {
			t.Errorf("expected PASS for %s, got %s", r.Name, r.Level)
		}
	}
}

func TestExitCode_AllPass(t *testing.T) {
	results := []CheckResult{
		{Level: LevelPass},
		{Level: LevelPass},
	}
	if ExitCode(results) != 0 {
		t.Error("expected exit 0 for all-pass")
	}
}

func TestExitCode_HasWarn(t *testing.T) {
	results := []CheckResult{
		{Level: LevelPass},
		{Level: LevelWarn},
	}
	if ExitCode(results) != 1 {
		t.Error("expected exit 1 for warn")
	}
}

func TestExitCode_HasFail(t *testing.T) {
	results := []CheckResult{
		{Level: LevelWarn},
		{Level: LevelFail},
	}
	if ExitCode(results) != 2 {
		t.Error("expected exit 2 for fail")
	}
}

func TestSummary_Counts(t *testing.T) {
	results := []CheckResult{
		{Level: LevelPass},
		{Level: LevelPass},
		{Level: LevelWarn},
		{Level: LevelFail},
	}
	p, w, f := Summary(results)
	if p != 2 || w != 1 || f != 1 {
		t.Errorf("expected 2/1/1, got %d/%d/%d", p, w, f)
	}
}

func TestSuite_EmptyChecks(t *testing.T) {
	s := NewSuite()
	results := s.Run()
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty suite, got %d", len(results))
	}
	if ExitCode(results) != 0 {
		t.Error("empty suite should exit 0")
	}
}
