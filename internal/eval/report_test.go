package eval

import (
	"strings"
	"testing"
)

func TestGenerateReport_Summary(t *testing.T) {
	results := []EvalResult{
		{EvalID: "e1", EvalName: "test1", Pass: true, Score: 1.0, Iterations: 1},
		{EvalID: "e2", EvalName: "test2", Pass: false, Score: 0.3, Iterations: 3},
		{EvalID: "e3", EvalName: "test3", Pass: true, Score: 0.9, Iterations: 2},
	}

	report := GenerateReport("run-001", results)

	if report.EvalCount != 3 {
		t.Errorf("EvalCount = %d, want 3", report.EvalCount)
	}
	if report.PassCount != 2 {
		t.Errorf("PassCount = %d, want 2", report.PassCount)
	}
	if report.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", report.FailCount)
	}
	if report.PassRate < 0.66 || report.PassRate > 0.67 {
		t.Errorf("PassRate = %f, want ~0.667", report.PassRate)
	}
}

func TestReport_ToMarkdown(t *testing.T) {
	results := []EvalResult{
		{EvalID: "e1", EvalName: "build-check", EvalType: EvalCapability, Pass: true, Score: 1.0, Iterations: 1},
	}

	report := GenerateReport("run-001", results)
	md := report.ToMarkdown()

	if !strings.Contains(md, "# Eval Report: run-001") {
		t.Error("missing report header")
	}
	if !strings.Contains(md, "build-check") {
		t.Error("missing eval name in results table")
	}
	if !strings.Contains(md, "PASS") {
		t.Error("missing PASS status")
	}
	if !strings.Contains(md, "100.0%") {
		t.Error("missing pass rate percentage")
	}
}

func TestReport_WithTrend_Improvement(t *testing.T) {
	results := []EvalResult{
		{Pass: true, Score: 1.0},
		{Pass: true, Score: 0.9},
	}

	report := GenerateReport("run-002", results)
	report.WithTrend(0.5)

	if report.TrendDelta < 0.49 || report.TrendDelta > 0.51 {
		t.Errorf("TrendDelta = %f, want ~0.5", report.TrendDelta)
	}

	md := report.ToMarkdown()
	if !strings.Contains(md, "improved") {
		t.Error("expected 'improved' in trend")
	}
}

func TestReport_WithTrend_Regression(t *testing.T) {
	results := []EvalResult{
		{Pass: false, Score: 0.2},
	}

	report := GenerateReport("run-003", results)
	report.WithTrend(0.9)

	if report.TrendDelta > -0.5 {
		t.Errorf("TrendDelta = %f, expected negative", report.TrendDelta)
	}

	md := report.ToMarkdown()
	if !strings.Contains(md, "regressed") {
		t.Error("expected 'regressed' in trend")
	}
}

func TestQualityBadge(t *testing.T) {
	tests := []struct {
		rate float64
		want string
	}{
		{0.95, "Platinum"},
		{0.85, "Gold"},
		{0.70, "Silver"},
		{0.55, "Bronze"},
		{0.30, "None"},
	}
	for _, tc := range tests {
		got := QualityBadge(tc.rate)
		if got != tc.want {
			t.Errorf("QualityBadge(%f) = %q, want %q", tc.rate, got, tc.want)
		}
	}
}
