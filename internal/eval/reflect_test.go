package eval

import (
	"testing"
)

// passingDef returns an EvalDef whose shell criterion always passes (command="true").
func passingDef() EvalDef {
	return EvalDef{
		ID:   "reflect-pass",
		Name: "reflect-pass",
		Type: EvalCapability,
		Task: "test",
		Criteria: []Criterion{
			{Name: "always-pass", GraderType: GraderShell, Command: "true"},
		},
	}
}

// failingDef returns an EvalDef whose shell criterion always fails (command="false").
func failingDef() EvalDef {
	return EvalDef{
		ID:   "reflect-fail",
		Name: "reflect-fail",
		Type: EvalCapability,
		Task: "test",
		Criteria: []Criterion{
			{Name: "always-fail", GraderType: GraderShell, Command: "false"},
		},
	}
}

func TestReflectAndRefine_StopsAtThreshold(t *testing.T) {
	t.Parallel()
	// A passing eval scores 1.0, which is >= threshold 0.9, so it should stop
	// after the first iteration.
	cfg := ReflectConfig{MaxIterations: 5, ScoreThreshold: 0.9}
	res := ReflectAndRefine(passingDef(), cfg)

	if res.Iterations != 1 {
		t.Errorf("Iterations = %d, want 1 (should stop at threshold)", res.Iterations)
	}
	if res.FinalScore < 0.999 {
		t.Errorf("FinalScore = %f, want ~1.0", res.FinalScore)
	}
	if len(res.History) != 1 {
		t.Errorf("len(History) = %d, want 1", len(res.History))
	}
}

func TestReflectAndRefine_RunsToMax(t *testing.T) {
	t.Parallel()
	// A failing eval never meets the threshold, so all iterations should run.
	cfg := ReflectConfig{MaxIterations: 3, ScoreThreshold: 0.9}
	res := ReflectAndRefine(failingDef(), cfg)

	if res.Iterations != 3 {
		t.Errorf("Iterations = %d, want 3", res.Iterations)
	}
	if len(res.History) != 3 {
		t.Errorf("len(History) = %d, want 3", len(res.History))
	}
	if res.FinalScore >= 0.9 {
		t.Errorf("FinalScore = %f, want < 0.9 for failing eval", res.FinalScore)
	}
}

func TestReflectAndRefine_DefaultConfig(t *testing.T) {
	t.Parallel()
	// Zero-value ReflectConfig must use defaults: MaxIterations=3, ScoreThreshold=0.9.
	// A failing eval should run exactly 3 times.
	var cfg ReflectConfig
	res := ReflectAndRefine(failingDef(), cfg)

	if res.Iterations != 3 {
		t.Errorf("Iterations = %d, want 3 (default MaxIterations)", res.Iterations)
	}
	if len(res.History) != 3 {
		t.Errorf("len(History) = %d, want 3", len(res.History))
	}
}

func TestReflectAndRefine_ImprovementTracked(t *testing.T) {
	t.Parallel()
	// A deterministic failing eval always returns the same score every iteration,
	// so Improved must be false (best == first, not strictly greater).
	cfg := ReflectConfig{MaxIterations: 3, ScoreThreshold: 0.9}
	res := ReflectAndRefine(failingDef(), cfg)

	if res.Improved {
		t.Errorf("Improved = true, want false for deterministic same-score runs")
	}
}

func TestReflectAndRefine_HistoryOrdering(t *testing.T) {
	t.Parallel()
	// History should contain results in run order; each entry has the correct
	// eval ID.
	cfg := ReflectConfig{MaxIterations: 2, ScoreThreshold: 0.5}
	res := ReflectAndRefine(failingDef(), cfg)

	for i, h := range res.History {
		if h.EvalID != "reflect-fail" {
			t.Errorf("History[%d].EvalID = %q, want reflect-fail", i, h.EvalID)
		}
	}
}

func TestReflectAndRefine_PassingImprovedFalse(t *testing.T) {
	t.Parallel()
	// A passing eval scores 1.0 on the first run, best == first == 1.0,
	// so Improved must be false (no improvement, already at max).
	cfg := ReflectConfig{MaxIterations: 3, ScoreThreshold: 0.9}
	res := ReflectAndRefine(passingDef(), cfg)

	if res.Improved {
		t.Error("Improved = true for a passing-on-first-run eval; expected false")
	}
}
