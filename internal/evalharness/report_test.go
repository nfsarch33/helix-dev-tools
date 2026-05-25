package evalharness

import (
	"strings"
	"testing"
)

func TestGenerateSprintReport_Basic(t *testing.T) {
	events := []AgentTraceEvent{
		{Event: "tool_call", LatencyMS: 100, Success: true},
		{Event: "tool_call", LatencyMS: 200, Success: true},
		{Event: "tool_call", LatencyMS: 150, Success: true, Error: "timeout"},
	}
	graders := AllGraders(DefaultGraderConfig())
	report := GenerateSprintReport("v11000", events, graders, DefaultGateConfig())

	if report.SprintID != "v11000" {
		t.Errorf("expected sprint ID v11000, got %s", report.SprintID)
	}
	if report.EventCount != 3 {
		t.Errorf("expected 3 events, got %d", report.EventCount)
	}
	graderCount := len(AllGraders(DefaultGraderConfig()))
	if len(report.GraderStats) != graderCount {
		t.Errorf("expected %d grader stats, got %d", graderCount, len(report.GraderStats))
	}
}

func TestGenerateSprintReport_Empty(t *testing.T) {
	graders := AllGraders(DefaultGraderConfig())
	report := GenerateSprintReport("v11000", nil, graders, DefaultGateConfig())

	if report.EventCount != 0 {
		t.Errorf("expected 0 events, got %d", report.EventCount)
	}
	if !report.Verdict.Pass {
		t.Error("empty report should pass")
	}
}

func TestFormatMarkdownReport_ContainsSprintID(t *testing.T) {
	events := []AgentTraceEvent{
		{Event: "tool_call", LatencyMS: 100, Success: true},
	}
	graders := AllGraders(DefaultGraderConfig())
	report := GenerateSprintReport("v11000", events, graders, DefaultGateConfig())
	md := FormatMarkdownReport(report)

	if !strings.Contains(md, "v11000") {
		t.Error("report should contain sprint ID")
	}
	if !strings.Contains(md, "PASS") {
		t.Error("passing report should contain PASS")
	}
	if !strings.Contains(md, "| latency |") {
		t.Error("report should contain grader table rows")
	}
}

func TestFormatMarkdownReport_ShowsFailures(t *testing.T) {
	events := []AgentTraceEvent{
		{Event: "tool_call", LatencyMS: 99999, Success: true},
	}
	graders := []DeterministicGrader{&LatencyGrader{MaxMS: 1000}}
	report := GenerateSprintReport("v11000", events, graders, GateConfig{MinPassRate: 0.99, MaxFailures: 0})
	md := FormatMarkdownReport(report)

	if !strings.Contains(md, "FAIL") {
		t.Error("failing report should contain FAIL")
	}
	if !strings.Contains(md, "Top Failures") {
		t.Error("failing report should list top failures")
	}
}

func TestGraderStats_PassRateCalculation(t *testing.T) {
	events := []AgentTraceEvent{
		{Event: "tool_call", LatencyMS: 100, Success: true},
		{Event: "tool_call", LatencyMS: 100, Success: true, Error: "fail"},
		{Event: "tool_call", LatencyMS: 100, Success: true},
	}
	graders := []DeterministicGrader{&ErrorRateGrader{}}
	report := GenerateSprintReport("v11000", events, graders, DefaultGateConfig())

	if len(report.GraderStats) != 1 {
		t.Fatalf("expected 1 grader stat, got %d", len(report.GraderStats))
	}
	stat := report.GraderStats[0]
	if stat.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", stat.Passed)
	}
	if stat.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stat.Failed)
	}
	expectedRate := 2.0 / 3.0
	if stat.PassRate < expectedRate-0.01 || stat.PassRate > expectedRate+0.01 {
		t.Errorf("expected pass rate ~%.2f, got %.2f", expectedRate, stat.PassRate)
	}
}
