package replicate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// integration test: build a faux cursor + claude home in a tmp dir,
// run the orchestrator, then assert the symlink layout.
func TestRun_Integration(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cursorHome := filepath.Join(tmp, ".cursor")
	cursorGKB := filepath.Join(tmp, "Code", "global-kb", "cursor-config")
	claudeHome := filepath.Join(tmp, ".claude")

	skillsRoot := filepath.Join(cursorGKB, "skills")
	agentsRoot := filepath.Join(cursorGKB, "agents")
	must := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(skillsRoot, "alpha", "SKILL.md"), "# alpha")
	must(filepath.Join(skillsRoot, "bravo", "SKILL.md"), "# bravo")
	must(filepath.Join(agentsRoot, "go-architect.md"), "x")
	must(filepath.Join(agentsRoot, "memory-ops.md"), "x")
	must(filepath.Join(cursorHome, "hooks.json"), `{"version":1}`)
	must(filepath.Join(cursorHome, "mcp.json"), `{"mcpServers":{"test-mcp":{"command":"x"},"context7":{"command":"npx"}}}`)
	if err := os.MkdirAll(claudeHome, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	out, err := Run(Options{
		CursorHome:     cursorHome,
		CursorGlobalKB: cursorGKB,
		ClaudeHome:     claudeHome,
		Out:            &buf,
	})
	if err != nil {
		t.Fatalf("Run failed: %v\noutput=%s", err, buf.String())
	}
	if len(out) == 0 {
		t.Fatal("expected at least one action")
	}

	// Skills symlinks present.
	for _, name := range []string{"alpha", "bravo"} {
		linkPath := filepath.Join(claudeHome, "skills", name)
		dest, rerr := os.Readlink(linkPath)
		if rerr != nil {
			t.Fatalf("skill %s missing: %v", name, rerr)
		}
		if !strings.HasSuffix(dest, filepath.Join("skills", name)) {
			t.Errorf("skill %s points at %q, expected suffix %q", name, dest, filepath.Join("skills", name))
		}
	}
	// Agents symlinks present.
	for _, name := range []string{"go-architect.md", "memory-ops.md"} {
		_, rerr := os.Readlink(filepath.Join(claudeHome, "agents", name))
		if rerr != nil {
			t.Fatalf("agent %s missing: %v", name, rerr)
		}
	}
	// hooks.json symlink.
	if _, err := os.Readlink(filepath.Join(claudeHome, "hooks.json")); err != nil {
		t.Fatalf("hooks.json symlink missing: %v", err)
	}
	// Filtered mcp.json present and does NOT contain test-mcp.
	mcpBytes, err := os.ReadFile(filepath.Join(claudeHome, "mcp.json"))
	if err != nil {
		t.Fatalf("mcp.json missing: %v", err)
	}
	if strings.Contains(string(mcpBytes), "test-mcp") {
		t.Fatalf("filtered mcp.json must drop test-mcp: %s", string(mcpBytes))
	}
	if !strings.Contains(string(mcpBytes), "context7") {
		t.Fatalf("filtered mcp.json must keep context7: %s", string(mcpBytes))
	}
}

func TestRun_RequiresClaudeHome(t *testing.T) {
	t.Parallel()
	_, err := Run(Options{})
	if err == nil {
		t.Fatal("expected error when ClaudeHome is empty")
	}
	if !strings.Contains(err.Error(), "ClaudeHome") {
		t.Fatalf("error should mention ClaudeHome, got %q", err.Error())
	}
}

func TestRun_DryRunDoesNotMutate(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cursorHome := filepath.Join(tmp, ".cursor")
	cursorGKB := filepath.Join(tmp, "Code", "global-kb", "cursor-config")
	claudeHome := filepath.Join(tmp, ".claude")
	must := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(cursorGKB, "skills", "alpha", "SKILL.md"), "# alpha")
	if err := os.MkdirAll(claudeHome, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Run(Options{
		CursorHome:     cursorHome,
		CursorGlobalKB: cursorGKB,
		ClaudeHome:     claudeHome,
		DryRun:         true,
		Out:            &buf,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(claudeHome, "skills", "alpha")); err == nil {
		t.Fatal("dry-run must not create skill symlink")
	}
}

// TestRun_DirSymlinkSinkSkipsCategory guards the foot-gun where the
// operator pre-symlinked ~/.claude/agents to the source agents dir. In
// that situation the orchestrator must NOT plan per-file symlinks (the
// applier would back up the source files themselves), and must instead
// emit one SKIP action so the operator sees what happened.
func TestRun_DirSymlinkSinkSkipsCategory(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cursorHome := filepath.Join(tmp, ".cursor")
	cursorGKB := filepath.Join(tmp, "Code", "global-kb", "cursor-config")
	claudeHome := filepath.Join(tmp, ".claude")
	srcAgents := filepath.Join(cursorGKB, "agents")
	for _, p := range []struct {
		path string
		body string
	}{
		{filepath.Join(srcAgents, "go-architect.md"), "x"},
		{filepath.Join(cursorHome, "hooks.json"), "{}"},
		{filepath.Join(cursorHome, "mcp.json"), `{"mcpServers":{}}`},
	} {
		if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p.path, []byte(p.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(claudeHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(srcAgents, filepath.Join(claudeHome, "agents")); err != nil {
		t.Fatalf("seed agents dir-symlink: %v", err)
	}

	var buf bytes.Buffer
	out, err := Run(Options{
		CursorHome:     cursorHome,
		CursorGlobalKB: cursorGKB,
		ClaudeHome:     claudeHome,
		Out:            &buf,
	})
	if err != nil {
		t.Fatalf("Run: %v\noutput=%s", err, buf.String())
	}

	var sawAgentsSkip bool
	for _, a := range out {
		if a.Op == OpSkip && a.Target == filepath.Join(claudeHome, "agents") {
			sawAgentsSkip = true
		}
		// The original source file must still exist after the run.
		if a.Op == OpBackup && strings.HasPrefix(a.Target, srcAgents) {
			t.Fatalf("dir-symlinked agents/ caused backup of source: %+v", a)
		}
	}
	if !sawAgentsSkip {
		t.Fatalf("expected a SKIP action targeting %s, got %+v", filepath.Join(claudeHome, "agents"), out)
	}
	if _, err := os.Stat(filepath.Join(srcAgents, "go-architect.md")); err != nil {
		t.Fatalf("source go-architect.md disappeared: %v", err)
	}
}

// TestRun_NoAgentsFlagSkipsAgents covers the explicit operator opt-out
// (--no-agents). Even when the sink is empty, --no-agents must produce
// no agent symlinks.
func TestRun_NoAgentsFlagSkipsAgents(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cursorHome := filepath.Join(tmp, ".cursor")
	cursorGKB := filepath.Join(tmp, "Code", "global-kb", "cursor-config")
	claudeHome := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(filepath.Join(cursorGKB, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cursorHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cursorGKB, "agents", "x.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cursorHome, "hooks.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cursorHome, "mcp.json"), []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(claudeHome, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Run(Options{
		CursorHome:     cursorHome,
		CursorGlobalKB: cursorGKB,
		ClaudeHome:     claudeHome,
		NoAgents:       true,
		Out:            &buf,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(claudeHome, "agents")); err == nil {
		t.Fatal("--no-agents must not create the agents dir")
	}
}

func TestRun_SkillsOnlyFlagFiltersOtherCategories(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cursorHome := filepath.Join(tmp, ".cursor")
	cursorGKB := filepath.Join(tmp, "Code", "global-kb", "cursor-config")
	claudeHome := filepath.Join(tmp, ".claude")
	must := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(cursorGKB, "skills", "alpha", "SKILL.md"), "# alpha")
	must(filepath.Join(cursorGKB, "agents", "x.md"), "x")
	must(filepath.Join(cursorHome, "hooks.json"), `{}`)
	must(filepath.Join(cursorHome, "mcp.json"), `{"mcpServers":{}}`)
	if err := os.MkdirAll(claudeHome, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Run(Options{
		CursorHome:     cursorHome,
		CursorGlobalKB: cursorGKB,
		ClaudeHome:     claudeHome,
		SkillsOnly:     true,
		Out:            &buf,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(claudeHome, "skills", "alpha")); err != nil {
		t.Fatalf("alpha skill should be symlinked: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(claudeHome, "agents", "x.md")); err == nil {
		t.Fatal("--skills-only must skip agents")
	}
	if _, err := os.Lstat(filepath.Join(claudeHome, "hooks.json")); err == nil {
		t.Fatal("--skills-only must skip hooks")
	}
	if _, err := os.Lstat(filepath.Join(claudeHome, "mcp.json")); err == nil {
		t.Fatal("--skills-only must skip mcp")
	}
}
