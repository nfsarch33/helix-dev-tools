package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestReplicateSubcommandRegistered guards against a regression where
// the replicate subcommand stops being wired into the root command.
// We assert membership on rootCmd directly so the test does not depend
// on argv parsing or filesystem state.
func TestReplicateSubcommandRegistered(t *testing.T) {
	t.Parallel()
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "replicate-cursor-to-claude-code" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("replicate-cursor-to-claude-code not registered on rootCmd")
	}
}

// TestReplicateDryRunSmoke drives the subcommand end-to-end against a
// synthetic HOME so we never touch the operator's real ~/.claude or
// ~/.cursor. The dry-run flag forbids the applier from writing, which
// the test cross-checks by asserting the sink directory is still empty
// after the run.
//
// macOS and Linux both honour the symlink semantics, so we run on both.
// Windows is not currently a target for this tool (the SOP and the
// underlying applier rely on POSIX symlinks); if a future port lands,
// add a -short skip here instead of running the test.
func TestReplicateDryRunSmoke(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("replicate-cursor-to-claude-code is POSIX-only today")
	}

	tmp := t.TempDir()
	cursorHome := filepath.Join(tmp, ".cursor")
	cursorKB := filepath.Join(tmp, "Code", "global-kb", "cursor-config")
	claudeHome := filepath.Join(tmp, ".claude")
	skillsRoot := filepath.Join(cursorKB, "skills")
	agentsRoot := filepath.Join(cursorKB, "agents")

	for _, d := range []string{
		filepath.Join(skillsRoot, "demo-skill"),
		agentsRoot,
		cursorHome,
		claudeHome,
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(
		filepath.Join(skillsRoot, "demo-skill", "SKILL.md"),
		[]byte("# demo\n"),
		0o644,
	); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(agentsRoot, "demo-agent.md"),
		[]byte("# agent\n"),
		0o644,
	); err != nil {
		t.Fatalf("write agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cursorHome, "hooks.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write hooks.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cursorHome, "mcp.json"), []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatalf("write mcp.json: %v", err)
	}

	t.Setenv("HOME", tmp)

	defer func() {
		replicateDryRun = false
		replicateSkillsOnly = false
		replicateAgentsOnly = false
		replicateNoMCP = false
		replicateNoHooks = false
	}()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"replicate-cursor-to-claude-code", "--dry-run"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute --dry-run: %v\noutput: %s", err, out.String())
	}

	got := out.String()
	if !strings.Contains(got, "demo-skill") {
		t.Fatalf("expected demo-skill in dry-run output, got:\n%s", got)
	}
	if !strings.Contains(got, "demo-agent.md") {
		t.Fatalf("expected demo-agent.md in dry-run output, got:\n%s", got)
	}

	for _, sub := range []string{"skills", "agents", "hooks.json", "mcp.json"} {
		_, err := os.Lstat(filepath.Join(claudeHome, sub))
		if err == nil {
			t.Fatalf("dry-run created %s", sub)
		}
	}
}
