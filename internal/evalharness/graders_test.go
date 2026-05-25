package evalharness

import (
	"encoding/json"
	"testing"
)

func TestLatencyGrader_Pass(t *testing.T) {
	g := &LatencyGrader{MaxMS: 5000}
	event := AgentTraceEvent{Event: "tool_call", LatencyMS: 1200}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass for latency 1200ms <= 5000ms, got fail: %s", result.Reason)
	}
	if result.Score != 1.0 {
		t.Errorf("expected score 1.0, got %f", result.Score)
	}
}

func TestLatencyGrader_Fail(t *testing.T) {
	g := &LatencyGrader{MaxMS: 2000}
	event := AgentTraceEvent{Event: "tool_call", LatencyMS: 4000}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail for latency 4000ms > 2000ms")
	}
	if result.Score >= 1.0 {
		t.Errorf("expected score < 1.0, got %f", result.Score)
	}
}

func TestLatencyGrader_SkipNonToolCall(t *testing.T) {
	g := &LatencyGrader{MaxMS: 5000}
	event := AgentTraceEvent{Event: "llm_complete", LatencyMS: 99999}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("non-tool_call events should always pass")
	}
}

func TestErrorRateGrader_NoError(t *testing.T) {
	g := &ErrorRateGrader{}
	event := AgentTraceEvent{Success: true}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass for successful event")
	}
}

func TestErrorRateGrader_WithError(t *testing.T) {
	g := &ErrorRateGrader{}
	event := AgentTraceEvent{Error: "timeout"}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail for event with error")
	}
}

func TestCoverageGrader_Pass(t *testing.T) {
	g := &CoverageGrader{MinCoverage: 0.70}
	event := AgentTraceEvent{Event: "test_run", Coverage: 0.85}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass for 85%% coverage >= 70%% min")
	}
}

func TestCoverageGrader_Fail(t *testing.T) {
	g := &CoverageGrader{MinCoverage: 0.70}
	event := AgentTraceEvent{Event: "test_run", Coverage: 0.55}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail for 55%% coverage < 70%% min")
	}
}

func TestCoverageGrader_SkipNonTestRun(t *testing.T) {
	g := &CoverageGrader{MinCoverage: 0.70}
	event := AgentTraceEvent{Event: "tool_call", Coverage: 0.10}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("non-test_run events should pass")
	}
}

func TestLintGrader_Clean(t *testing.T) {
	g := &LintGrader{}
	clean := true
	event := AgentTraceEvent{LintClean: &clean}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass for clean lint")
	}
}

func TestLintGrader_Dirty(t *testing.T) {
	g := &LintGrader{}
	dirty := false
	event := AgentTraceEvent{LintClean: &dirty}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail for dirty lint")
	}
}

func TestLintGrader_NilSkip(t *testing.T) {
	g := &LintGrader{}
	event := AgentTraceEvent{}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("nil lint data should pass")
	}
}

func TestTestPassGrader_Pass(t *testing.T) {
	g := &TestPassGrader{}
	pass := true
	event := AgentTraceEvent{TestPass: &pass}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass when tests pass")
	}
}

func TestTestPassGrader_Fail(t *testing.T) {
	g := &TestPassGrader{}
	fail := false
	event := AgentTraceEvent{TestPass: &fail}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail when tests fail")
	}
}

func TestTokenEfficiencyGrader_WithinBudget(t *testing.T) {
	g := &TokenEfficiencyGrader{MaxTokens: 50000}
	event := AgentTraceEvent{TokensUsed: 25000}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass for 25000 tokens <= 50000 max")
	}
}

func TestTokenEfficiencyGrader_OverBudget(t *testing.T) {
	g := &TokenEfficiencyGrader{MaxTokens: 50000}
	event := AgentTraceEvent{TokensUsed: 75000}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail for 75000 tokens > 50000 max")
	}
}

func TestTokenEfficiencyGrader_NoData(t *testing.T) {
	g := &TokenEfficiencyGrader{MaxTokens: 50000}
	event := AgentTraceEvent{}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("no token data should pass")
	}
}

func TestCommitMessageGrader_Conventional(t *testing.T) {
	g := &CommitMessageGrader{RequireConventional: true}
	event := AgentTraceEvent{Event: "commit", Raw: json.RawMessage(`"feat(eval): add grader pipeline"`)}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass for conventional commit: %s", result.Reason)
	}
}

func TestCommitMessageGrader_NonConventional(t *testing.T) {
	g := &CommitMessageGrader{RequireConventional: true}
	event := AgentTraceEvent{Event: "commit", Raw: json.RawMessage(`"Updated some stuff"`)}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail for non-conventional commit")
	}
}

func TestCommitMessageGrader_SkipNonCommit(t *testing.T) {
	g := &CommitMessageGrader{RequireConventional: true}
	event := AgentTraceEvent{Event: "tool_call"}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("non-commit events should pass")
	}
}

func TestToolCoverageGrader_Pass(t *testing.T) {
	g := &ToolCoverageGrader{MinCoverage: 0.60}
	event := AgentTraceEvent{Event: "session_summary", ToolsTotal: 10, ToolsExercised: 8}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass for 80%% tool coverage >= 60%% min: %s", result.Reason)
	}
	if result.Score < 0.79 || result.Score > 0.81 {
		t.Errorf("expected score ~0.80, got %f", result.Score)
	}
}

func TestToolCoverageGrader_Fail(t *testing.T) {
	g := &ToolCoverageGrader{MinCoverage: 0.60}
	event := AgentTraceEvent{Event: "session_summary", ToolsTotal: 10, ToolsExercised: 3}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail for 30%% tool coverage < 60%% min")
	}
}

func TestToolCoverageGrader_NoData(t *testing.T) {
	g := &ToolCoverageGrader{MinCoverage: 0.60}
	event := AgentTraceEvent{Event: "tool_call"}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("events without tool coverage data should pass")
	}
}

func TestToolCoverageGrader_ZeroTools(t *testing.T) {
	g := &ToolCoverageGrader{MinCoverage: 0.60}
	event := AgentTraceEvent{Event: "session_summary", ToolsTotal: 0}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("zero registered tools should pass")
	}
}

func TestCompletionRateGrader_Pass(t *testing.T) {
	g := &CompletionRateGrader{MinRate: 0.80}
	event := AgentTraceEvent{Event: "session_summary", TasksTotal: 10, TasksCompleted: 9}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass for 90%% completion >= 80%% min: %s", result.Reason)
	}
	if result.Score < 0.89 || result.Score > 0.91 {
		t.Errorf("expected score ~0.90, got %f", result.Score)
	}
}

func TestCompletionRateGrader_Fail(t *testing.T) {
	g := &CompletionRateGrader{MinRate: 0.80}
	event := AgentTraceEvent{Event: "session_summary", TasksTotal: 10, TasksCompleted: 5}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail for 50%% completion < 80%% min")
	}
}

func TestCompletionRateGrader_NoData(t *testing.T) {
	g := &CompletionRateGrader{MinRate: 0.80}
	event := AgentTraceEvent{Event: "tool_call"}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("events without task data should pass")
	}
}

func TestRegressionGrader_NoRegression(t *testing.T) {
	g := &RegressionGrader{MaxDelta: 0.10}
	event := AgentTraceEvent{Event: "eval_comparison", BaselineScore: 0.90, CurrentScore: 0.92}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass when score improved: %s", result.Reason)
	}
}

func TestRegressionGrader_SmallRegression(t *testing.T) {
	g := &RegressionGrader{MaxDelta: 0.10}
	event := AgentTraceEvent{Event: "eval_comparison", BaselineScore: 0.90, CurrentScore: 0.85}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("expected pass for 0.05 regression within 0.10 threshold: %s", result.Reason)
	}
}

func TestRegressionGrader_LargeRegression(t *testing.T) {
	g := &RegressionGrader{MaxDelta: 0.10}
	event := AgentTraceEvent{Event: "eval_comparison", BaselineScore: 0.90, CurrentScore: 0.70}
	result := g.Grade(event)
	if result.Pass {
		t.Errorf("expected fail for 0.20 regression exceeding 0.10 threshold")
	}
}

func TestRegressionGrader_NoBaseline(t *testing.T) {
	g := &RegressionGrader{MaxDelta: 0.10}
	event := AgentTraceEvent{Event: "tool_call"}
	result := g.Grade(event)
	if !result.Pass {
		t.Errorf("events without baseline should pass")
	}
}

func TestAllGraders_Returns9(t *testing.T) {
	graders := AllGraders(DefaultGraderConfig())
	if len(graders) != 9 {
		t.Errorf("expected 9 graders, got %d", len(graders))
	}
	names := make(map[string]bool)
	for _, g := range graders {
		names[g.Name()] = true
	}
	expected := []string{"latency", "error_rate", "coverage", "lint", "test_pass", "token_efficiency", "tool_coverage", "completion_rate", "regression"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing grader: %s", name)
		}
	}
}

func TestADR065Graders_Returns6(t *testing.T) {
	graders := ADR065Graders(DefaultGraderConfig())
	if len(graders) != 6 {
		t.Errorf("expected 6 ADR-065 graders, got %d", len(graders))
	}
	names := make(map[string]bool)
	for _, g := range graders {
		names[g.Name()] = true
	}
	expected := []string{"latency", "error_rate", "tool_coverage", "token_efficiency", "completion_rate", "regression"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing ADR-065 grader: %s", name)
		}
	}
}

func TestIsConventionalCommit(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"feat(eval): add graders", true},
		{"fix: resolve crash", true},
		{"docs: update README", true},
		{"chore(deps): bump go", true},
		{"Updated stuff", false},
		{"", false},
		{"random message", false},
	}
	for _, tc := range cases {
		got := isConventionalCommit(tc.msg)
		if got != tc.want {
			t.Errorf("isConventionalCommit(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}
