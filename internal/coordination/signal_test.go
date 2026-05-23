package coordination

import (
	"strings"
	"testing"
)

func TestIsValidSignalType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"active-state", true},
		{"task-dispatch", true},
		{"decision", true},
		{"blocker", true},
		{"completed", true},
		{"unknown", false},
		{"", false},
		{"ACTIVE-STATE", false},
	}
	for _, tc := range tests {
		if got := IsValidSignalType(tc.input); got != tc.want {
			t.Errorf("IsValidSignalType(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestSignalMem0Text(t *testing.T) {
	tests := []struct {
		name   string
		signal Signal
		want   string
	}{
		{
			name: "active-state",
			signal: Signal{
				Type:    SignalActiveState,
				Machine: "wsl",
				Message: "v10.0 fuzz targets for selfhealv4",
			},
			want: "wsl working on: v10.0 fuzz targets for selfhealv4",
		},
		{
			name: "task-dispatch with target",
			signal: Signal{
				Type:      SignalTaskDispatch,
				Machine:   "wsl",
				TargetFor: "macbook",
				Message:   "Review and merge feat/v10-fuzz-evolver-integration",
				Priority:  "high",
			},
			want: "Task for macbook: Review and merge feat/v10-fuzz-evolver-integration Priority: high.",
		},
		{
			name: "task-dispatch without target",
			signal: Signal{
				Type:    SignalTaskDispatch,
				Machine: "wsl",
				Message: "Review the PR",
			},
			want: "Task: Review the PR",
		},
		{
			name: "blocker",
			signal: Signal{
				Type:    SignalBlocker,
				Machine: "macos",
				Message: "Need ADR-0011 resolved before proceeding",
			},
			want: "Blocker: Need ADR-0011 resolved before proceeding",
		},
		{
			name: "decision",
			signal: Signal{
				Type:    SignalDecision,
				Machine: "wsl",
				Message: "Reverting to base Qwen3.5-27B",
			},
			want: "Decision: Reverting to base Qwen3.5-27B",
		},
		{
			name: "completed",
			signal: Signal{
				Type:    SignalCompleted,
				Machine: "wsl",
				Message: "Fuzz targets added for 6 packages",
			},
			want: "Completed: Fuzz targets added for 6 packages",
		},
		{
			name: "normal priority omitted",
			signal: Signal{
				Type:     SignalActiveState,
				Machine:  "wsl",
				Message:  "Working on tests",
				Priority: "normal",
			},
			want: "wsl working on: Working on tests",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.signal.Mem0Text()
			if got != tc.want {
				t.Errorf("Mem0Text() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSignalMem0Metadata(t *testing.T) {
	s := Signal{
		Type:      SignalTaskDispatch,
		Machine:   "wsl",
		TargetFor: "macbook",
		Priority:  "high",
		Sprint:    "154",
		Metadata:  map[string]string{"repo": "helixon-ops"},
	}
	meta := s.Mem0Metadata()

	checks := map[string]string{
		"type":       "task-dispatch",
		"machine":    "wsl",
		"target_for": "macbook",
		"priority":   "high",
		"sprint":     "154",
		"repo":       "helixon-ops",
	}
	for key, want := range checks {
		if got := meta[key]; got != want {
			t.Errorf("Mem0Metadata()[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestMem0MetadataOmitsEmpty(t *testing.T) {
	s := Signal{
		Type:    SignalActiveState,
		Machine: "wsl",
		Message: "working",
	}
	meta := s.Mem0Metadata()
	if _, ok := meta["target_for"]; ok {
		t.Error("target_for should be omitted when empty")
	}
	if _, ok := meta["priority"]; ok {
		t.Error("priority should be omitted when empty")
	}
	if _, ok := meta["sprint"]; ok {
		t.Error("sprint should be omitted when empty")
	}
}

func TestLocalMachine(t *testing.T) {
	machine := LocalMachine()
	if machine == "" {
		t.Error("LocalMachine() returned empty string")
	}
	valid := map[string]bool{"macos": true, "wsl": true, "linux": true, "windows": true}
	if !valid[machine] {
		t.Errorf("LocalMachine() = %q, want a recognised platform", machine)
	}
}

func TestRenderHandoffSection_Empty(t *testing.T) {
	result := RenderHandoffSection(nil)
	if result != "" {
		t.Errorf("RenderHandoffSection(nil) = %q, want empty string", result)
	}
}

func TestRenderHandoffSection_GroupedByType(t *testing.T) {
	signals := []Signal{
		{Type: SignalActiveState, Machine: "wsl", Message: "Working on fuzz targets"},
		{Type: SignalDecision, Machine: "wsl", Message: "Revert to base Qwen3.5-27B"},
		{Type: SignalTaskDispatch, Machine: "wsl", TargetFor: "macbook", Message: "Review PR", Priority: "high"},
		{Type: SignalCompleted, Machine: "wsl", Message: "Added 12 fuzz targets"},
		{Type: SignalBlocker, Machine: "macos", Message: "Need ADR-0011 resolved"},
	}

	result := RenderHandoffSection(signals)

	sectionOrder := []string{"## Task Summary", "## Decisions Made", "## Blockers", "## Delegated Tasks", "## Completed Items"}
	var positions []int
	for _, sec := range sectionOrder {
		pos := strings.Index(result, sec)
		if pos < 0 {
			t.Fatalf("expected section %q not found in:\n%s", sec, result)
		}
		positions = append(positions, pos)
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Errorf("section %q (pos %d) should appear after %q (pos %d)",
				sectionOrder[i], positions[i], sectionOrder[i-1], positions[i-1])
		}
	}

	if !strings.Contains(result, "*(for macbook)*") {
		t.Error("expected target annotation '*(for macbook)*'")
	}
	if !strings.Contains(result, "[priority: high]") {
		t.Error("expected priority annotation '[priority: high]'")
	}
	if !strings.Contains(result, "1. Working on fuzz targets") {
		t.Error("expected numbered list item for active-state signal")
	}
}

func TestRenderHandoffSection_SingleType(t *testing.T) {
	signals := []Signal{
		{Type: SignalBlocker, Machine: "wsl", Message: "GPU OOM"},
		{Type: SignalBlocker, Machine: "wsl", Message: "Docker not responding"},
	}

	result := RenderHandoffSection(signals)
	if !strings.Contains(result, "## Blockers") {
		t.Fatal("expected '## Blockers' section")
	}
	if strings.Contains(result, "## Task Summary") {
		t.Error("should not contain empty sections")
	}
	if !strings.Contains(result, "1. GPU OOM") {
		t.Error("expected first blocker numbered 1")
	}
	if !strings.Contains(result, "2. Docker not responding") {
		t.Error("expected second blocker numbered 2")
	}
}

func TestAppIDConstant(t *testing.T) {
	if AppID != "cursor-coordination" {
		t.Errorf("AppID = %q, want 'cursor-coordination'", AppID)
	}
}
