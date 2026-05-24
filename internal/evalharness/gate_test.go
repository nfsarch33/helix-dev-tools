package evalharness

import (
	"testing"
)

func TestEvaluateGate_AllPass(t *testing.T) {
	events := []AgentTraceEvent{
		{Event: "tool_call", LatencyMS: 100, Success: true},
		{Event: "tool_call", LatencyMS: 200, Success: true},
	}
	graders := AllGraders(DefaultGraderConfig())
	cfg := DefaultGateConfig()

	verdict := EvaluateGate(events, graders, cfg)
	if !verdict.Pass {
		t.Errorf("expected gate PASS for normal events, got FAIL: %d failures", verdict.FailCount)
	}
	if verdict.PassRate < 1.0 {
		t.Errorf("expected 100%% pass rate, got %.1f%%", verdict.PassRate*100)
	}
}

func TestEvaluateGate_HighErrorRate(t *testing.T) {
	events := make([]AgentTraceEvent, 20)
	for i := range events {
		events[i] = AgentTraceEvent{Event: "tool_call", Success: true, LatencyMS: 100}
		if i < 15 {
			events[i].Error = "timeout"
			events[i].Success = false
		}
	}
	graders := AllGraders(DefaultGraderConfig())
	cfg := GateConfig{MinPassRate: 0.80, MaxFailures: 5}

	verdict := EvaluateGate(events, graders, cfg)
	if verdict.Pass {
		t.Errorf("expected gate FAIL with many errors, got PASS")
	}
}

func TestEvaluateGate_EmptyEvents(t *testing.T) {
	graders := AllGraders(DefaultGraderConfig())
	cfg := DefaultGateConfig()

	verdict := EvaluateGate(nil, graders, cfg)
	if !verdict.Pass {
		t.Errorf("empty events should pass (no failures)")
	}
}

func TestEvaluateGate_MaxFailuresExceeded(t *testing.T) {
	events := []AgentTraceEvent{
		{Event: "tool_call", LatencyMS: 99999, Success: true},
		{Event: "tool_call", LatencyMS: 99999, Success: true},
		{Event: "tool_call", LatencyMS: 99999, Success: true},
		{Event: "tool_call", LatencyMS: 99999, Success: true},
		{Event: "tool_call", LatencyMS: 99999, Success: true},
		{Event: "tool_call", LatencyMS: 99999, Success: true},
	}
	graders := []DeterministicGrader{&LatencyGrader{MaxMS: 5000}}
	cfg := GateConfig{MinPassRate: 0.50, MaxFailures: 3}

	verdict := EvaluateGate(events, graders, cfg)
	if verdict.Pass {
		t.Errorf("expected FAIL when max_failures=3 but got %d failures", verdict.FailCount)
	}
}

func TestFormatGateVerdict_Pass(t *testing.T) {
	v := GateVerdict{Pass: true, PassRate: 0.95, FailCount: 2, TotalCount: 40}
	output := FormatGateVerdict(v)
	if output == "" {
		t.Error("expected non-empty output")
	}
	if !contains(output, "PASS") {
		t.Errorf("expected PASS in output, got: %s", output)
	}
}

func TestFormatGateVerdict_Fail(t *testing.T) {
	v := GateVerdict{
		Pass: false, PassRate: 0.60, FailCount: 8, TotalCount: 20,
		Failures: []GradeResult{
			{GraderName: "latency", Reason: "too slow"},
			{GraderName: "error_rate", Reason: "errors detected"},
		},
	}
	output := FormatGateVerdict(v)
	if !contains(output, "FAIL") {
		t.Errorf("expected FAIL in output")
	}
	if !contains(output, "latency") {
		t.Errorf("expected failure details in output")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstr(s, substr))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
