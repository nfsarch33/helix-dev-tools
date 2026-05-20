package sprintdispatch

import (
	"strings"
	"testing"
)

func TestGenerateKickoff_BasicFields(t *testing.T) {
	tickets := []Ticket{
		{ID: "T-6800-1", Title: "Implement health check", Status: "backlog"},
		{ID: "T-6800-2", Title: "Add retry logic", Status: "backlog"},
	}
	cfg := DispatchConfig{
		SprintID:  "v6800",
		Target:    TargetClaudeCode,
		Workspace: "/home/user/engram",
		Hours:     10,
	}
	prompt := GenerateKickoff(cfg, tickets)

	if !strings.Contains(prompt, "v6800") {
		t.Errorf("kickoff must contain sprint ID, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "T-6800-1") {
		t.Error("kickoff must list ticket IDs")
	}
	if !strings.Contains(prompt, "T-6800-2") {
		t.Error("kickoff must list all tickets")
	}
	if !strings.Contains(prompt, "Implement health check") {
		t.Error("kickoff must include ticket titles")
	}
}

func TestGenerateKickoff_EmptyTickets(t *testing.T) {
	cfg := DispatchConfig{
		SprintID: "v7000",
		Target:   TargetCodex,
	}
	prompt := GenerateKickoff(cfg, nil)
	if !strings.Contains(prompt, "v7000") {
		t.Error("kickoff must still contain sprint ID even with no tickets")
	}
	if !strings.Contains(prompt, "No tickets") {
		t.Error("kickoff must indicate no tickets assigned")
	}
}

func TestBuildCommand_ClaudeCode(t *testing.T) {
	cfg := DispatchConfig{
		SprintID:  "v6800",
		Target:    TargetClaudeCode,
		Workspace: "/home/user/engram",
		Hours:     10,
	}
	args := BuildCommand(cfg, "test prompt content")
	if len(args) == 0 {
		t.Fatal("BuildCommand returned empty args")
	}
	if args[0] != "claude" {
		t.Errorf("expected claude binary, got %q", args[0])
	}
	found := false
	for _, a := range args {
		if a == "-p" {
			found = true
		}
	}
	if !found {
		t.Error("claude command must include -p flag")
	}
}

func TestBuildCommand_Codex(t *testing.T) {
	cfg := DispatchConfig{
		SprintID: "v5032",
		Target:   TargetCodex,
		Hours:    8,
	}
	args := BuildCommand(cfg, "test prompt")
	if len(args) == 0 {
		t.Fatal("BuildCommand returned empty args")
	}
	if args[0] != "codex" {
		t.Errorf("expected codex binary, got %q", args[0])
	}
}

func TestBuildCommand_UnknownTarget(t *testing.T) {
	cfg := DispatchConfig{
		SprintID: "v9999",
		Target:   "unknown-agent",
	}
	args := BuildCommand(cfg, "prompt")
	if args != nil {
		t.Errorf("unknown target should return nil, got %v", args)
	}
}

func TestDispatchConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     DispatchConfig
		wantErr bool
	}{
		{"valid claude", DispatchConfig{SprintID: "v6800", Target: TargetClaudeCode, Hours: 1}, false},
		{"valid codex", DispatchConfig{SprintID: "v5032", Target: TargetCodex, Hours: 4}, false},
		{"missing sprint", DispatchConfig{Target: TargetClaudeCode, Hours: 1}, true},
		{"missing target", DispatchConfig{SprintID: "v6800", Hours: 1}, true},
		{"zero hours", DispatchConfig{SprintID: "v6800", Target: TargetClaudeCode}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
