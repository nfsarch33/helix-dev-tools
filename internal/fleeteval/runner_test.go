package fleeteval

import (
	"context"
	"fmt"
	"testing"
)

type mockLLM struct {
	responses map[string]string
	err       error
}

func (m *mockLLM) Complete(_ context.Context, _, userPrompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	for key, resp := range m.responses {
		if len(userPrompt) > 0 && contains(userPrompt, key) {
			return resp, nil
		}
	}
	return "default response", nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestRunner_RunAll_SinglePass(t *testing.T) {
	llm := &mockLLM{
		responses: map[string]string{
			"eval-01": "Echo Test Title",
		},
	}
	runner := NewRunner(llm, RunnerConfig{
		SystemPrompt:   "You are a test agent.",
		Model:          "test-model",
		TimeoutSeconds: 10,
	}, nil)

	tasks := []Task{{
		ID:              "eval-01",
		Level:           1,
		Title:           "Echo test",
		Description:     "Echo back the title",
		ExpectedPattern: `^.+$`,
		Grading:         Grading{MaxScore: 10, PassThreshold: 5},
	}}

	report, err := runner.RunAll(context.Background(), tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.TotalTasks != 1 {
		t.Errorf("expected 1 task, got %d", report.TotalTasks)
	}
	if report.PassCount != 1 {
		t.Errorf("expected 1 pass, got %d", report.PassCount)
	}
	if report.TotalScore != 10 {
		t.Errorf("expected score 10, got %d", report.TotalScore)
	}
	if report.Verdict != "GREEN" {
		t.Errorf("expected GREEN verdict, got %s", report.Verdict)
	}
}

func TestRunner_RunAll_LLMError(t *testing.T) {
	llm := &mockLLM{err: fmt.Errorf("connection refused")}
	runner := NewRunner(llm, RunnerConfig{
		SystemPrompt:   "test",
		Model:          "test",
		TimeoutSeconds: 5,
	}, nil)

	tasks := []Task{{
		ID:              "eval-err",
		Level:           1,
		Title:           "Error task",
		Description:     "This will error",
		ExpectedPattern: `.*`,
		Grading:         Grading{MaxScore: 10, PassThreshold: 5},
	}}

	report, err := runner.RunAll(context.Background(), tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", report.ErrorCount)
	}
	if report.PassCount != 0 {
		t.Errorf("expected 0 pass, got %d", report.PassCount)
	}
	if report.TotalScore != 0 {
		t.Errorf("expected score 0, got %d", report.TotalScore)
	}
}

func TestRunner_RunAll_MultipleTasks(t *testing.T) {
	llm := &mockLLM{
		responses: map[string]string{
			"eval-01": "Echo Title",
			"eval-02": "2026-05-27T17:00",
			"eval-03": "no digits here",
		},
	}
	runner := NewRunner(llm, RunnerConfig{
		SystemPrompt:   "test",
		Model:          "test",
		TimeoutSeconds: 10,
	}, nil)

	tasks := []Task{
		{ID: "eval-01", Level: 1, Title: "Echo", Description: "Echo", ExpectedPattern: `^.+$`, Grading: Grading{MaxScore: 10, PassThreshold: 5}},
		{ID: "eval-02", Level: 1, Title: "Time", Description: "Timestamp", ExpectedPattern: `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}`, Grading: Grading{MaxScore: 10, PassThreshold: 7}},
		{ID: "eval-03", Level: 2, Title: "Digits", Description: "Has digits", ExpectedPattern: `\d+`, Grading: Grading{MaxScore: 10, PassThreshold: 5}},
	}

	report, err := runner.RunAll(context.Background(), tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.TotalTasks != 3 {
		t.Errorf("expected 3 tasks, got %d", report.TotalTasks)
	}
	if report.PassCount != 2 {
		t.Errorf("expected 2 passes (eval-01, eval-02), got %d", report.PassCount)
	}
	if report.FailCount != 1 {
		t.Errorf("expected 1 fail (eval-03), got %d", report.FailCount)
	}
}

func TestVerdictFromScore(t *testing.T) {
	cases := []struct {
		total, max int
		want       string
	}{
		{80, 100, "GREEN"},
		{90, 100, "GREEN"},
		{50, 100, "YELLOW"},
		{79, 100, "YELLOW"},
		{49, 100, "RED"},
		{0, 100, "RED"},
		{0, 0, "no_tasks"},
	}
	for _, tc := range cases {
		got := verdictFromScore(tc.total, tc.max)
		if got != tc.want {
			t.Errorf("verdictFromScore(%d, %d) = %q, want %q", tc.total, tc.max, got, tc.want)
		}
	}
}

func TestBuildEvalPrompt(t *testing.T) {
	task := Task{ID: "eval-01", Level: 2, Title: "Count lines", Description: "Count lines in /etc/hosts"}
	prompt := buildEvalPrompt(task)
	if !containsSubstr(prompt, "eval-01") {
		t.Error("expected task ID in prompt")
	}
	if !containsSubstr(prompt, "Count lines") {
		t.Error("expected title in prompt")
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if got := truncate(short, 100); got != short {
		t.Errorf("expected %q, got %q", short, got)
	}
	long := "a very long string that exceeds the limit"
	got := truncate(long, 10)
	if len(got) > 30 {
		t.Errorf("expected truncated string, got len %d", len(got))
	}
}
