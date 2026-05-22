package sprinteval

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComputeCompletion(t *testing.T) {
	tests := []struct {
		name      string
		tickets   []TicketSnapshot
		wantRate  float64
		wantDone  int
		wantStart int
	}{
		{
			name:     "empty",
			tickets:  nil,
			wantRate: 0,
		},
		{
			name: "all done",
			tickets: []TicketSnapshot{
				{ID: "1", Status: "done"},
				{ID: "2", Status: "done"},
			},
			wantRate:  1.0,
			wantDone:  2,
			wantStart: 2,
		},
		{
			name: "mixed",
			tickets: []TicketSnapshot{
				{ID: "1", Status: "done"},
				{ID: "2", Status: "in_progress"},
				{ID: "3", Status: "backlog"},
				{ID: "4", Status: "done"},
			},
			wantRate:  0.5,
			wantDone:  2,
			wantStart: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rate, done, started := computeCompletion(tc.tickets)
			if math.Abs(rate-tc.wantRate) > 0.001 {
				t.Errorf("rate = %f, want %f", rate, tc.wantRate)
			}
			if done != tc.wantDone {
				t.Errorf("done = %d, want %d", done, tc.wantDone)
			}
			if started != tc.wantStart {
				t.Errorf("started = %d, want %d", started, tc.wantStart)
			}
		})
	}
}

func TestComputeCoverage(t *testing.T) {
	tickets := []TicketSnapshot{
		{ID: "1", Status: "done"},
		{ID: "2", Status: "in_progress"},
		{ID: "3", Status: "backlog"},
		{ID: "4", Status: "review"},
		{ID: "5", Status: "ready"},
	}

	coverage := computeCoverage(tickets)
	expected := 3.0 / 5.0
	if math.Abs(coverage-expected) > 0.001 {
		t.Errorf("coverage = %f, want %f", coverage, expected)
	}
}

func TestComputeErrorRate(t *testing.T) {
	events := []AgentraceEvent{
		{Event: "tool_call", Tool: "search"},
		{Event: "tool_call", Tool: "claim", Error: "timeout"},
		{Event: "llm_complete"},
		{Event: "tool_call", Tool: "read", Error: "not found"},
		{Event: "llm_complete"},
	}

	rate, total, errors := computeErrorRate(events)
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if errors != 2 {
		t.Errorf("errors = %d, want 2", errors)
	}
	expected := 2.0 / 5.0
	if math.Abs(rate-expected) > 0.001 {
		t.Errorf("rate = %f, want %f", rate, expected)
	}
}

func TestComputeToolReliability(t *testing.T) {
	events := []AgentraceEvent{
		{Event: "tool_call", Tool: "a"},
		{Event: "tool_call", Tool: "b"},
		{Event: "tool_call", Tool: "c", Error: "fail"},
		{Event: "llm_complete"},
	}

	rate, total, failed := computeToolReliability(events)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
	expected := 2.0 / 3.0
	if math.Abs(rate-expected) > 0.001 {
		t.Errorf("rate = %f, want %f", rate, expected)
	}
}

func TestComputeToolReliabilityNoToolCalls(t *testing.T) {
	rate, total, _ := computeToolReliability(nil)
	if rate != 1.0 {
		t.Errorf("rate = %f, want 1.0 when no tool calls", rate)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}

func TestComputeEstimationAccuracy(t *testing.T) {
	estimates := map[string]Estimate{
		"phase1": {
			Corrected: 10 * time.Minute,
			Actual:    12 * time.Minute,
		},
		"phase2": {
			Corrected: 5 * time.Minute,
			Actual:    5 * time.Minute,
		},
	}

	accuracy := computeEstimationAccuracy(estimates)
	phase1Acc := 1.0 - (2.0 / 10.0)
	phase2Acc := 1.0
	expected := (phase1Acc + phase2Acc) / 2.0

	if math.Abs(accuracy-expected) > 0.001 {
		t.Errorf("accuracy = %f, want %f", accuracy, expected)
	}
}

func TestComputeEstimationAccuracyEmpty(t *testing.T) {
	accuracy := computeEstimationAccuracy(nil)
	if accuracy != 1.0 {
		t.Errorf("accuracy = %f, want 1.0 for empty estimates", accuracy)
	}
}

func TestComputeTokenUsage(t *testing.T) {
	events := []AgentraceEvent{
		{TokensIn: 100, TokensOut: 50},
		{TokensIn: 200, TokensOut: 100},
		{TokensIn: 150, TokensOut: 75},
	}

	total, avg := computeTokenUsage(events, 3)
	if total != 675 {
		t.Errorf("total = %d, want 675", total)
	}
	if avg != 225 {
		t.Errorf("avg = %d, want 225", avg)
	}
}

func TestComputeTestResults(t *testing.T) {
	tests := []TestResult{
		{Pass: true, Elapsed: 0.5},
		{Pass: true, Elapsed: 1.2},
		{Pass: false, Elapsed: 0.3, Output: "assertion failed"},
		{Pass: false},
	}

	passed, failed, skipped := computeTestResults(tests)
	if passed != 2 {
		t.Errorf("passed = %d, want 2", passed)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
}

func TestDefaultWeights(t *testing.T) {
	w := DefaultWeights()
	sum := w.CompletionRate + w.EstimationAccuracy + w.Coverage + w.ErrorRate + w.ToolReliability
	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("weights sum = %f, want 1.0", sum)
	}
}

func TestGradeScore(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.95, "EXCELLENT"},
		{0.90, "EXCELLENT"},
		{0.85, "GREEN"},
		{0.80, "GREEN"},
		{0.70, "AMBER"},
		{0.50, "RED"},
	}

	for _, tc := range cases {
		got := gradeScore(tc.score)
		if got != tc.want {
			t.Errorf("gradeScore(%f) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestEstimationRatio(t *testing.T) {
	ratio := EstimationRatio(10*time.Minute, 5*time.Minute)
	if math.Abs(ratio-2.0) > 0.001 {
		t.Errorf("ratio = %f, want 2.0", ratio)
	}

	ratio = EstimationRatio(5*time.Minute, 10*time.Minute)
	if math.Abs(ratio-0.5) > 0.001 {
		t.Errorf("ratio = %f, want 0.5", ratio)
	}
}

func TestQualityTrend(t *testing.T) {
	if trend := QualityTrend([]float64{0.60, 0.70, 0.82}); trend != "improving" {
		t.Errorf("trend = %q, want improving", trend)
	}
	if trend := QualityTrend([]float64{0.80, 0.75, 0.60}); trend != "degrading" {
		t.Errorf("trend = %q, want degrading", trend)
	}
	if trend := QualityTrend([]float64{0.80, 0.81}); trend != "stable" {
		t.Errorf("trend = %q, want stable", trend)
	}
	if trend := QualityTrend([]float64{0.80}); trend != "insufficient_data" {
		t.Errorf("trend = %q, want insufficient_data", trend)
	}
}

func TestSprintEvalRun(t *testing.T) {
	se := New(DefaultWeights(), nil)

	input := SprintInput{
		SprintID:   "v-test-001",
		SprintName: "Test Sprint",
		Tickets: []TicketSnapshot{
			{ID: "t1", Status: "done", Title: "Task 1"},
			{ID: "t2", Status: "done", Title: "Task 2"},
			{ID: "t3", Status: "in_progress", Title: "Task 3"},
			{ID: "t4", Status: "backlog", Title: "Task 4"},
		},
		Estimates: map[string]Estimate{
			"phase1": {Corrected: 10 * time.Minute, Actual: 12 * time.Minute},
		},
	}

	report, err := se.Run(input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if report.SprintID != "v-test-001" {
		t.Errorf("SprintID = %q, want v-test-001", report.SprintID)
	}
	if report.QualityScore <= 0 {
		t.Errorf("QualityScore = %f, want > 0", report.QualityScore)
	}
	if report.QualityGrade == "" {
		t.Error("QualityGrade should not be empty")
	}
	if len(report.Breakdown) == 0 {
		t.Error("Breakdown should not be empty")
	}
	if report.Metrics.TotalTickets != 4 {
		t.Errorf("TotalTickets = %d, want 4", report.Metrics.TotalTickets)
	}
	if report.Metrics.CompletedTickets != 2 {
		t.Errorf("CompletedTickets = %d, want 2", report.Metrics.CompletedTickets)
	}
}

func TestSprintEvalRunMissingID(t *testing.T) {
	se := New(DefaultWeights(), nil)
	_, err := se.Run(SprintInput{})
	if err == nil {
		t.Error("Run with empty sprint_id should fail")
	}
}

func TestWriteReport(t *testing.T) {
	se := New(DefaultWeights(), nil)

	report := &SprintReport{
		SprintID:     "v-test-write",
		SprintName:   "Write Test",
		EvaluatedAt:  time.Now(),
		QualityScore: 0.85,
		QualityGrade: "GREEN",
		Metrics: SprintMetrics{
			CompletionRate: 0.85,
			TotalTickets:   10,
		},
		Breakdown: []MetricDetail{
			{Name: "Completion Rate", Value: 0.85, Target: 0.80, Pass: true},
		},
		Findings: []string{"All metrics within target ranges"},
	}

	dir := filepath.Join(t.TempDir(), "sprint-evals")
	path, err := se.WriteReport(report, dir)
	if err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	content := string(data)
	if !contains(content, "Sprint Eval: v-test-write") {
		t.Error("report should contain sprint ID header")
	}
	if !contains(content, "0.85") {
		t.Error("report should contain quality score")
	}
	if !contains(content, "GREEN") {
		t.Error("report should contain grade")
	}
}

func TestWriteJSON(t *testing.T) {
	se := New(DefaultWeights(), nil)

	report := &SprintReport{
		SprintID:     "v-test-json",
		QualityScore: 0.82,
		QualityGrade: "GREEN",
	}

	dir := filepath.Join(t.TempDir(), "sprint-evals")
	path, err := se.WriteJSON(report, dir)
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JSON: %v", err)
	}

	var parsed SprintReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.SprintID != "v-test-json" {
		t.Errorf("parsed SprintID = %q, want v-test-json", parsed.SprintID)
	}
}

func TestComputeMetrics(t *testing.T) {
	tickets := []TicketSnapshot{
		{ID: "1", Status: "done"},
		{ID: "2", Status: "done"},
		{ID: "3", Status: "in_progress"},
	}

	events := []AgentraceEvent{
		{Event: "tool_call", Tool: "search", TokensIn: 100, TokensOut: 50},
		{Event: "tool_call", Tool: "write", TokensIn: 200, TokensOut: 100, Error: "disk full"},
		{Event: "llm_complete", TokensIn: 500, TokensOut: 300},
	}

	tests := []TestResult{
		{Pass: true, Elapsed: 0.5},
		{Pass: false, Elapsed: 0.1, Output: "fail"},
	}

	estimates := map[string]Estimate{
		"p1": {Corrected: 10 * time.Minute, Actual: 10 * time.Minute},
	}

	m := ComputeMetrics(tickets, events, tests, estimates)

	if m.TotalTickets != 3 {
		t.Errorf("TotalTickets = %d, want 3", m.TotalTickets)
	}
	if m.CompletedTickets != 2 {
		t.Errorf("CompletedTickets = %d, want 2", m.CompletedTickets)
	}
	if m.TotalToolCalls != 2 {
		t.Errorf("TotalToolCalls = %d, want 2", m.TotalToolCalls)
	}
	if m.FailedToolCalls != 1 {
		t.Errorf("FailedToolCalls = %d, want 1", m.FailedToolCalls)
	}
	if m.TestsPassed != 1 {
		t.Errorf("TestsPassed = %d, want 1", m.TestsPassed)
	}
	if m.TestsFailed != 1 {
		t.Errorf("TestsFailed = %d, want 1", m.TestsFailed)
	}
	if m.TotalTokens != 1250 {
		t.Errorf("TotalTokens = %d, want 1250", m.TotalTokens)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
