package eval

import (
	"encoding/json"
	"testing"
)

func TestEvalDef_Validate_RequiresID(t *testing.T) {
	def := EvalDef{Name: "test", Criteria: []Criterion{{Name: "c1"}}}
	if err := def.Validate(); err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestEvalDef_Validate_RequiresCriteria(t *testing.T) {
	def := EvalDef{ID: "e1", Name: "test"}
	if err := def.Validate(); err == nil {
		t.Fatal("expected error for empty criteria")
	}
}

func TestEvalDef_Validate_SetsDefaults(t *testing.T) {
	def := EvalDef{ID: "e1", Name: "test", Criteria: []Criterion{{Name: "c1"}}}
	if err := def.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.MaxIterations != 3 {
		t.Errorf("MaxIterations = %d, want 3", def.MaxIterations)
	}
	if def.TimeoutSec != 300 {
		t.Errorf("TimeoutSec = %d, want 300", def.TimeoutSec)
	}
}

func TestEvalResult_Serialization(t *testing.T) {
	r := EvalResult{
		EvalID:   "e1",
		EvalName: "test",
		Pass:     true,
		Score:    0.95,
		Criteria: []CriterionResult{{Name: "c1", Pass: true, Score: 1.0}},
	}

	data, err := r.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	var decoded EvalResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.EvalID != "e1" {
		t.Errorf("EvalID = %q, want e1", decoded.EvalID)
	}
	if decoded.Score != 0.95 {
		t.Errorf("Score = %f, want 0.95", decoded.Score)
	}
}

func TestEvalResult_ToNDJSON(t *testing.T) {
	r := EvalResult{EvalID: "e1", Pass: true}
	ndjson := r.ToNDJSON()
	if ndjson == "" {
		t.Fatal("expected non-empty NDJSON")
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(ndjson), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestCodeGrader_PatternMatch(t *testing.T) {
	g := &CodeGrader{Pattern: "func main"}
	score, _, err := g.Grade("package main\nfunc main() {}")
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if score != 1.0 {
		t.Errorf("score = %f, want 1.0", score)
	}
}

func TestCodeGrader_PatternMiss(t *testing.T) {
	g := &CodeGrader{Pattern: "func missing"}
	score, _, err := g.Grade("package main\nfunc main() {}")
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if score != 0 {
		t.Errorf("score = %f, want 0", score)
	}
}

func TestCodeGrader_EmptyPattern(t *testing.T) {
	g := &CodeGrader{}
	_, _, err := g.Grade("anything")
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}
}

func TestShellGrader_SuccessCommand(t *testing.T) {
	g := &ShellGrader{Command: "true"}
	score, _, err := g.Grade("")
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if score != 1.0 {
		t.Errorf("score = %f, want 1.0", score)
	}
}

func TestShellGrader_FailCommand(t *testing.T) {
	g := &ShellGrader{Command: "false"}
	score, _, err := g.Grade("")
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if score != 0 {
		t.Errorf("score = %f, want 0", score)
	}
}

func TestNewGrader_DefaultsToCode(t *testing.T) {
	g := NewGrader(Criterion{Pattern: "test"})
	if _, ok := g.(*CodeGrader); !ok {
		t.Error("expected CodeGrader as default")
	}
}

func TestRunner_CapabilityEval_AllPass(t *testing.T) {
	runner := NewRunner()
	def := EvalDef{
		ID:   "e1",
		Name: "all-pass",
		Type: EvalCapability,
		Task: "func main() {}",
		Criteria: []Criterion{
			{Name: "has_func", GraderType: GraderCode, Pattern: "func main"},
		},
	}

	result := runner.Run(def)
	if !result.Pass {
		t.Errorf("expected pass, got fail: %+v", result)
	}
	if result.Score < 0.99 {
		t.Errorf("score = %f, want >= 0.99", result.Score)
	}
	if result.Iterations != 1 {
		t.Errorf("iterations = %d, want 1 (should pass first try)", result.Iterations)
	}
}

func TestRunner_CapabilityEval_AllFail(t *testing.T) {
	runner := NewRunner()
	def := EvalDef{
		ID:   "e2",
		Name: "all-fail",
		Type: EvalCapability,
		Task: "nothing useful here",
		Criteria: []Criterion{
			{Name: "has_func", GraderType: GraderCode, Pattern: "func main"},
		},
		MaxIterations: 2,
	}

	result := runner.Run(def)
	if result.Pass {
		t.Error("expected fail")
	}
	if result.Iterations != 2 {
		t.Errorf("iterations = %d, want 2", result.Iterations)
	}
}

func TestRunner_InvalidDef(t *testing.T) {
	runner := NewRunner()
	def := EvalDef{}
	result := runner.Run(def)
	if result.Pass {
		t.Error("expected fail for invalid def")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestPassAtK_SuccessInThree(t *testing.T) {
	results := []EvalResult{
		{Pass: false},
		{Pass: false},
		{Pass: true},
	}
	if got := PassAtK(results, 3); got != 1.0 {
		t.Errorf("pass@3 = %f, want 1.0", got)
	}
	if got := PassAtK(results, 1); got != 0 {
		t.Errorf("pass@1 = %f, want 0", got)
	}
}

func TestPassPowerK_AllPass(t *testing.T) {
	results := []EvalResult{
		{Pass: true},
		{Pass: true},
		{Pass: true},
	}
	if got := PassPowerK(results, 3); got != 1.0 {
		t.Errorf("pass^3 = %f, want 1.0", got)
	}
}

func TestPassPowerK_OneFail(t *testing.T) {
	results := []EvalResult{
		{Pass: true},
		{Pass: false},
		{Pass: true},
	}
	if got := PassPowerK(results, 3); got != 0 {
		t.Errorf("pass^3 = %f, want 0", got)
	}
}

func TestPassRate(t *testing.T) {
	results := []EvalResult{
		{Pass: true},
		{Pass: false},
		{Pass: true},
		{Pass: true},
	}
	if got := PassRate(results); got != 0.75 {
		t.Errorf("pass rate = %f, want 0.75", got)
	}
}

func TestAverageScore(t *testing.T) {
	results := []EvalResult{
		{Score: 0.8},
		{Score: 1.0},
		{Score: 0.6},
	}
	avg := AverageScore(results)
	if avg < 0.79 || avg > 0.81 {
		t.Errorf("avg score = %f, want ~0.8", avg)
	}
}

func TestPassAtK_EmptyResults(t *testing.T) {
	if got := PassAtK(nil, 3); got != 0 {
		t.Errorf("expected 0 for empty results, got %f", got)
	}
}

func TestPassRate_EmptyResults(t *testing.T) {
	if got := PassRate(nil); got != 0 {
		t.Errorf("expected 0 for empty results, got %f", got)
	}
}
