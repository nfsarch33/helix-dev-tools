package replicate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedTime() time.Time {
	return time.Date(2026, 4, 29, 0, 30, 0, 0, time.UTC)
}

func newApplier(dryRun bool) *Applier {
	return &Applier{
		DryRun:      dryRun,
		FilteredMCP: []byte(`{"mcpServers":{}}` + "\n"),
		Now:         fixedTime,
	}
}

func TestApplier_FreshSymlink(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "skill-src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "claude", "skills", "alpha")

	plan := Plan{Skills: []Action{{Op: OpSymlink, Source: src, Target: target}}}
	out, err := newApplier(false).Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Op != OpSymlink {
		t.Fatalf("expected SYMLINK, got %+v", out)
	}
	resolved, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != src {
		t.Fatalf("symlink wrong: want=%q got=%q", src, resolved)
	}
}

func TestApplier_IdempotentSkip(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "skill-src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "claude", "skills", "alpha")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, target); err != nil {
		t.Fatal(err)
	}

	plan := Plan{Skills: []Action{{Op: OpSymlink, Source: src, Target: target}}}
	out, err := newApplier(false).Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Op != OpSkip {
		t.Fatalf("expected SKIP for already-correct symlink, got %+v", out[0])
	}
}

func TestApplier_BackupExistingNonSymlink(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "skill-src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "claude", "skills", "alpha")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a regular file at target — it should be preserved as a backup.
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := Plan{Skills: []Action{{Op: OpSymlink, Source: src, Target: target}}}
	out, err := newApplier(false).Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Op != OpBackup {
		t.Fatalf("expected BACKUP, got %+v", out[0])
	}
	// Symlink now in place.
	resolved, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != src {
		t.Fatalf("post-backup symlink wrong: want=%q got=%q", src, resolved)
	}
	// Backup file present with the original content.
	bak := target + ".bak.20260429T003000Z"
	got, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("backup content corrupted: %q", string(got))
	}
}

func TestApplier_BackupReplacesDivergentSymlink(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "skill-src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	wrongSrc := filepath.Join(tmp, "wrong-src")
	if err := os.MkdirAll(wrongSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "claude", "skills", "alpha")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wrongSrc, target); err != nil {
		t.Fatal(err)
	}

	plan := Plan{Skills: []Action{{Op: OpSymlink, Source: src, Target: target}}}
	out, err := newApplier(false).Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Op != OpBackup {
		t.Fatalf("expected BACKUP (divergent symlink), got %+v", out[0])
	}
	resolved, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != src {
		t.Fatalf("post-backup symlink wrong: want=%q got=%q", src, resolved)
	}
}

func TestApplier_DryRunMakesNoChanges(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "skill-src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "claude", "skills", "alpha")

	plan := Plan{Skills: []Action{{Op: OpSymlink, Source: src, Target: target}}}
	out, err := newApplier(true).Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Op != OpSymlink {
		t.Fatalf("dry-run should still report SYMLINK in plan, got %+v", out[0])
	}
	if _, err := os.Lstat(target); err == nil {
		t.Fatalf("dry-run must not create target, but it exists")
	}
}

func TestApplier_RewriteFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	target := filepath.Join(tmp, "claude", "mcp.json")
	plan := Plan{MCP: []Action{{Op: OpRewrite, Source: "(filtered)", Target: target, Reason: "filtered cursor mcp.json"}}}
	out, err := newApplier(false).Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Op != OpRewrite {
		t.Fatalf("expected REWRITE, got %+v", out[0])
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"mcpServers":{}`) {
		t.Fatalf("rewrite content unexpected: %q", string(got))
	}
}

// TestApplier_RewriteIdempotent guards the byte-equal short-circuit so
// re-running the orchestrator on a settled tree produces zero disk
// mutations for mcp.json.
func TestApplier_RewriteIdempotent(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	target := filepath.Join(tmp, "claude", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"mcpServers":{}}` + "\n")
	if err := os.WriteFile(target, body, 0o600); err != nil {
		t.Fatal(err)
	}
	statBefore, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	mtimeBefore := statBefore.ModTime()

	plan := Plan{MCP: []Action{{Op: OpRewrite, Source: "(filtered)", Target: target, Reason: "filtered cursor mcp.json"}}}
	out, err := newApplier(false).Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Op != OpSkip {
		t.Fatalf("byte-identical mcp.json should produce SKIP, got %+v", out[0])
	}

	statAfter, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !statAfter.ModTime().Equal(mtimeBefore) {
		t.Fatalf("idempotent rewrite touched mtime: before=%v after=%v",
			mtimeBefore, statAfter.ModTime())
	}
}

func TestApplier_OpErrorBubbles(t *testing.T) {
	t.Parallel()
	plan := Plan{Skills: []Action{{Op: OpError, Reason: "boom"}}}
	out, err := newApplier(false).Apply(plan)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error containing 'boom', got %v", err)
	}
	if out[0].Op != OpError {
		t.Fatalf("expected error action passthrough, got %+v", out[0])
	}
}

func TestSampleExisting_DiscoversSkillsAndAgents(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	skills := filepath.Join(tmp, "claude", "skills")
	agents := filepath.Join(tmp, "claude", "agents")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	srcSkill := filepath.Join(tmp, "src", "alpha")
	if err := os.MkdirAll(srcSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(srcSkill, filepath.Join(skills, "alpha")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agents, "go-architect.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := SampleExisting(skills, agents, "", "")
	if got.LinkResolves[filepath.Join(skills, "alpha")] != srcSkill {
		t.Fatalf("symlink not resolved: %v", got.LinkResolves)
	}
	if got.LinkResolves[filepath.Join(agents, "go-architect.md")] != "" {
		t.Fatalf("non-symlink agent should resolve to empty string: %v", got.LinkResolves)
	}
}
