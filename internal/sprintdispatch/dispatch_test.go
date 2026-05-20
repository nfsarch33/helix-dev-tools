package sprintdispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAgent(t *testing.T) {
	_, err := ValidateAgent("codex")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateAgent("nope")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestBuild_CodexCommand(t *testing.T) {
	dir := t.TempDir()
	kickoff := filepath.Join(dir, "kickoff.md")
	if err := os.WriteFile(kickoff, []byte("# Kickoff\nRun sprint v7100.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Build(Spec{
		Agent:       AgentCodex,
		KickoffPath: kickoff,
		SprintID:    "v7100",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Command, "codex exec") {
		t.Fatalf("missing codex exec: %s", result.Command)
	}
	if !strings.Contains(result.Command, "</dev/null") {
		t.Fatalf("missing stdin close: %s", result.Command)
	}
	if !strings.Contains(result.Command, "v7100") {
		t.Fatalf("missing sprint id: %s", result.Command)
	}
	if len(result.ExecArgv) < 4 || result.ExecArgv[0] != "codex" {
		t.Fatalf("unexpected exec argv: %v", result.ExecArgv)
	}
}

func TestBuild_ClaudeTemplate(t *testing.T) {
	dir := t.TempDir()
	kickoff := filepath.Join(dir, "kickoff.md")
	if err := os.WriteFile(kickoff, []byte("kickoff body"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Build(Spec{
		Agent:       AgentClaudeCode,
		KickoffPath: kickoff,
		SprintID:    "v7100",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Command, "claude -p") {
		t.Fatalf("missing claude -p: %s", result.Command)
	}
	if strings.Contains(result.Command, "OPENAI") || strings.Contains(result.Command, "API_KEY") {
		t.Fatalf("must not embed secret env names: %s", result.Command)
	}
}
