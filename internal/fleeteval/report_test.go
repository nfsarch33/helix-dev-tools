package fleeteval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteNDJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "results.ndjson")

	results := []Result{
		{TaskID: "eval-01", Level: 1, Score: 10, MaxScore: 10, Pass: true, DurationMS: 500, Timestamp: time.Now()},
		{TaskID: "eval-02", Level: 2, Score: 0, MaxScore: 10, Pass: false, Error: "timeout", DurationMS: 5000, Timestamp: time.Now()},
	}

	if err := WriteNDJSON(path, results); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 NDJSON lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], `"eval-01"`) {
		t.Error("first line should contain eval-01")
	}
	if !strings.Contains(lines[1], `"timeout"`) {
		t.Error("second line should contain timeout error")
	}
}

func TestWriteNDJSON_Append(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "append.ndjson")

	r1 := []Result{{TaskID: "a", Level: 1, Score: 5, MaxScore: 10, Timestamp: time.Now()}}
	r2 := []Result{{TaskID: "b", Level: 2, Score: 8, MaxScore: 10, Timestamp: time.Now()}}

	if err := WriteNDJSON(path, r1); err != nil {
		t.Fatal(err)
	}
	if err := WriteNDJSON(path, r2); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines after append, got %d", len(lines))
	}
}

func TestFormatMarkdown(t *testing.T) {
	report := &RunReport{
		RunID:         "test-run",
		Model:         "test-model",
		Timestamp:     time.Date(2026, 5, 27, 17, 0, 0, 0, time.UTC),
		TotalTasks:    2,
		TotalScore:    15,
		MaxScore:      20,
		PassCount:     1,
		FailCount:     1,
		PassRate:      0.5,
		AvgDurationMS: 1000,
		Verdict:       "YELLOW",
		Results: []Result{
			{TaskID: "eval-01", Level: 1, Title: "Echo", Score: 10, MaxScore: 10, Pass: true, PatternMatch: true, DurationMS: 500},
			{TaskID: "eval-02", Level: 2, Title: "Fail", Score: 5, MaxScore: 10, Pass: false, PatternMatch: true, DurationMS: 1500, Response: "bad output"},
		},
	}

	md := FormatMarkdown(report)

	checks := []string{
		"# Fleet Eval Report: test-run",
		"test-model",
		"YELLOW",
		"15 / 20",
		"75.0%",
		"eval-01",
		"eval-02",
		"Failure Details",
	}
	for _, want := range checks {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestFilterFailures(t *testing.T) {
	results := []Result{
		{TaskID: "a", Pass: true},
		{TaskID: "b", Pass: false},
		{TaskID: "c", Pass: true, Error: ""},
		{TaskID: "d", Pass: false, Error: "broke"},
	}
	failures := filterFailures(results)
	if len(failures) != 2 {
		t.Errorf("expected 2 failures, got %d", len(failures))
	}
}
