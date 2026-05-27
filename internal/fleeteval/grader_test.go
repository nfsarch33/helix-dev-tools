package fleeteval

import "testing"

func TestMatchesPattern_Simple(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		resp    string
		want    bool
	}{
		{"any non-empty", `^.+$`, "hello world", true},
		{"empty fails", `^.+$`, "", false},
		{"digits", `\d+`, "there are 42 lines", true},
		{"no digits", `\d+`, "no numbers here", false},
		{"iso timestamp", `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}`, "2026-05-27T17:00+10:00", true},
		{"func sig", `func\s+[A-Z]`, "func SprintStatus(id string) error", true},
		{"case insensitive", `(?i)(mcp|server)`, "This is an MCP server", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MatchesPattern(tc.pattern, tc.resp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("MatchesPattern(%q, %q) = %v, want %v", tc.pattern, tc.resp, got, tc.want)
			}
		})
	}
}

func TestMatchesPattern_InvalidRegex(t *testing.T) {
	_, err := MatchesPattern("[invalid", "test")
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestGradeResponse_NoRubric(t *testing.T) {
	task := Task{
		ID:              "eval-01",
		ExpectedPattern: `^.+$`,
		Grading: Grading{
			MaxScore:      10,
			PassThreshold: 5,
		},
	}
	score, details, err := GradeResponse(task, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 10 {
		t.Errorf("expected score 10 for matching pattern with no rubric, got %d", score)
	}
	if len(details) != 1 {
		t.Errorf("expected 1 detail entry, got %d", len(details))
	}
}

func TestGradeResponse_PatternMismatch(t *testing.T) {
	task := Task{
		ID:              "eval-02",
		ExpectedPattern: `\d{4}-\d{2}-\d{2}`,
		Grading:         Grading{MaxScore: 10, PassThreshold: 7},
	}
	score, _, err := GradeResponse(task, "no timestamp here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0 {
		t.Errorf("expected score 0 for pattern mismatch, got %d", score)
	}
}

func TestGradeResponse_WithRubric(t *testing.T) {
	task := Task{
		ID:              "eval-05",
		ExpectedPattern: `(?i)(mcp|server)`,
		Grading: Grading{
			MaxScore:      10,
			PassThreshold: 6,
			QualityRubric: []RubricEntry{
				{Metric: "accuracy", Weight: 0.5, Description: "correct purpose"},
				{Metric: "conciseness", Weight: 0.3, Description: "2-3 sentences"},
				{Metric: "comprehension", Weight: 0.2, Description: "mentions MCP"},
			},
		},
	}
	response := "This file is an MCP server entry point that registers SprintBoard tools and starts the stdio transport."
	score, details, err := GradeResponse(task, response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score <= 0 {
		t.Errorf("expected positive score for good response, got %d", score)
	}
	if len(details) != 3 {
		t.Errorf("expected 3 detail entries, got %d", len(details))
	}
}

func TestGradeResponse_InvalidPattern(t *testing.T) {
	task := Task{
		ID:              "bad",
		ExpectedPattern: "[broken",
		Grading:         Grading{MaxScore: 10},
	}
	_, _, err := GradeResponse(task, "test")
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestGradeResponse_ZeroMaxScore(t *testing.T) {
	task := Task{
		ID:              "eval-z",
		ExpectedPattern: `.*`,
		Grading:         Grading{MaxScore: 0},
	}
	score, _, err := GradeResponse(task, "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 10 {
		t.Errorf("expected default max_score 10, got %d", score)
	}
}

func TestCleanResponse_ThinkBlock(t *testing.T) {
	raw := `<think>
Some reasoning here about the task...
Let me think step by step.
</think>
2026-05-27T17:00:00+10:00`
	got := CleanResponse(raw)
	if got != "2026-05-27T17:00:00+10:00" {
		t.Errorf("expected cleaned timestamp, got %q", got)
	}
}

func TestCleanResponse_UnclosedThinkBlock(t *testing.T) {
	raw := `<think>
Some reasoning that never closes...
Still thinking...
`
	got := CleanResponse(raw)
	if got != "" {
		t.Errorf("expected empty after stripping unclosed think block, got %q", got)
	}
}

func TestCleanResponse_ThinkThenAnswer(t *testing.T) {
	raw := `<think>
Planning my answer...
</think>

func CountMatches(ctx context.Context, items []string) (int, error)`
	got := CleanResponse(raw)
	want := "func CountMatches(ctx context.Context, items []string) (int, error)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanResponse_ProtocolWrapping(t *testing.T) {
	raw := `task_claim(ticket_id="eval-01", agent_id="fleet-agent", reason="echo")
fleet-agent-v18200-healthy
task_complete(ticket_id="eval-01", agent_id="fleet-agent", evidence="done")`
	got := CleanResponse(raw)
	if got != "fleet-agent-v18200-healthy" {
		t.Errorf("expected stripped response, got %q", got)
	}
}

func TestCleanResponse_CodeFences(t *testing.T) {
	raw := "```go\nfunc CountMatches(ctx context.Context, items []string) (int, error)\n```"
	got := CleanResponse(raw)
	want := "func CountMatches(ctx context.Context, items []string) (int, error)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanResponse_Empty(t *testing.T) {
	got := CleanResponse("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestCleanResponse_NoArtifacts(t *testing.T) {
	raw := "This is a clean response."
	got := CleanResponse(raw)
	if got != raw {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("run the command", []string{"command", "tool"}) {
		t.Error("expected true for 'command' in string")
	}
	if containsAny("hello world", []string{"foo", "bar"}) {
		t.Error("expected false for no match")
	}
}
